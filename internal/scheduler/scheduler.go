package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"flowpilot/internal/models"
)

type TaskSubmitter interface {
	SubmitScheduledTask(ctx context.Context, schedule models.Schedule) error
}

type ScheduleDB interface {
	ListDueSchedules(ctx context.Context, now time.Time) ([]models.Schedule, error)
	ListSchedules(ctx context.Context) ([]models.Schedule, error)
	UpdateScheduleRun(ctx context.Context, id string, lastRun, nextRun time.Time) error
	GetRecordedFlow(ctx context.Context, id string) (*models.RecordedFlow, error)
}

type Scheduler struct {
	db        ScheduleDB
	submitter TaskSubmitter
	interval  time.Duration
	stopCh    chan struct{}
	mu        sync.Mutex
	running   bool
	logf      func(format string, args ...any)
}

func New(db ScheduleDB, submitter TaskSubmitter, interval time.Duration) *Scheduler {
	return &Scheduler{
		db:        db,
		submitter: submitter,
		interval:  interval,
		stopCh:    make(chan struct{}),
		logf:      log.Printf,
	}
}

func (s *Scheduler) logError(format string, args ...any) {
	if s.logf != nil {
		s.logf(format, args...)
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.Recover(ctx)
	go s.loop(ctx)
}

// Recover queries all enabled schedules and immediately submits any whose
// next_run is in the past (i.e. missed while the application was offline).
func (s *Scheduler) Recover(ctx context.Context) {
	schedules, err := s.db.ListSchedules(ctx)
	if err != nil {
		s.logError("scheduler: recover: list schedules: %v", err)
		return
	}

	now := time.Now()
	recovered := 0
	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		if sched.NextRunAt.IsZero() || !sched.NextRunAt.Before(now) {
			continue
		}

		if submitErr := s.submitter.SubmitScheduledTask(ctx, sched); submitErr != nil {
			s.logError("scheduler: recover: submit schedule %s: %v", sched.ID, submitErr)
			continue
		}

		cronSched, err := ParseCron(sched.CronExpr)
		if err != nil {
			s.logError("scheduler: recover: parse cron for schedule %s: %v", sched.ID, err)
			continue
		}
		nextRun := cronSched.Next(now)
		if err := s.db.UpdateScheduleRun(ctx, sched.ID, now, nextRun); err != nil {
			s.logError("scheduler: recover: update schedule %s run: %v", sched.ID, err)
		}
		recovered++
	}

	if recovered > 0 {
		s.logError("scheduler: recovered %d missed schedule(s)", recovered)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.tick(ctx)

	for {
		select {
		case <-ticker.C:
			s.tick(ctx)
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()
	due, err := s.db.ListDueSchedules(ctx, now)
	if err != nil {
		s.logError("scheduler: list due schedules: %v", err)
		return
	}

	for _, sched := range due {
		cronSched, err := ParseCron(sched.CronExpr)
		if err != nil {
			s.logError("scheduler: parse cron for schedule %s: %v", sched.ID, err)
			continue
		}

		if submitErr := s.submitter.SubmitScheduledTask(ctx, sched); submitErr != nil {
			s.logError("scheduler: submit schedule %s: %v", sched.ID, submitErr)
			continue
		}

		nextRun := cronSched.Next(now)
		if err := s.db.UpdateScheduleRun(ctx, sched.ID, now, nextRun); err != nil {
			s.logError("scheduler: update schedule %s run: %v", sched.ID, err)
		}
	}
}

type CronSchedule struct {
	minutes     []int
	hours       []int
	daysOfMonth []int
	months      []int
	daysOfWeek  []int
}

func ParseCron(expr string) (*CronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("parse cron: expected 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("parse cron minute: %w", err)
	}
	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("parse cron hour: %w", err)
	}
	daysOfMonth, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("parse cron day-of-month: %w", err)
	}
	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("parse cron month: %w", err)
	}
	daysOfWeek, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("parse cron day-of-week: %w", err)
	}

	return &CronSchedule{
		minutes:     minutes,
		hours:       hours,
		daysOfMonth: daysOfMonth,
		months:      months,
		daysOfWeek:  daysOfWeek,
	}, nil
}

func parseField(field string, min, max int) ([]int, error) {
	var result []int
	parts := strings.Split(field, ",")
	for _, part := range parts {
		vals, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}
	return dedupSort(result, min, max), nil
}

func parsePart(part string, min, max int) ([]int, error) {
	if strings.Contains(part, "/") {
		return parseStep(part, min, max)
	}
	if strings.Contains(part, "-") {
		return parseRange(part, min, max)
	}
	if part == "*" {
		return rangeSlice(min, max), nil
	}
	v, err := strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid value %q: %w", part, err)
	}
	if v < min || v > max {
		return nil, fmt.Errorf("value %d out of range [%d-%d]", v, min, max)
	}
	return []int{v}, nil
}

func parseStep(part string, min, max int) ([]int, error) {
	tokens := strings.SplitN(part, "/", 2)
	step, err := strconv.Atoi(tokens[1])
	if err != nil || step <= 0 {
		return nil, fmt.Errorf("invalid step %q", tokens[1])
	}

	var start, end int
	if tokens[0] == "*" {
		start = min
		end = max
	} else if strings.Contains(tokens[0], "-") {
		r, err := parseRange(tokens[0], min, max)
		if err != nil {
			return nil, err
		}
		if len(r) == 0 {
			return nil, fmt.Errorf("empty range in step")
		}
		start = r[0]
		end = r[len(r)-1]
	} else {
		v, err := strconv.Atoi(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("invalid step base %q: %w", tokens[0], err)
		}
		start = v
		end = max
	}

	var result []int
	for i := start; i <= end; i += step {
		result = append(result, i)
	}
	return result, nil
}

func parseRange(part string, min, max int) ([]int, error) {
	tokens := strings.SplitN(part, "-", 2)
	low, err := strconv.Atoi(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("invalid range start %q: %w", tokens[0], err)
	}
	high, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("invalid range end %q: %w", tokens[1], err)
	}
	if low < min || high > max || low > high {
		return nil, fmt.Errorf("range %d-%d out of bounds [%d-%d]", low, high, min, max)
	}
	return rangeSlice(low, high), nil
}

func rangeSlice(low, high int) []int {
	result := make([]int, 0, high-low+1)
	for i := low; i <= high; i++ {
		result = append(result, i)
	}
	return result
}

func dedupSort(vals []int, min, max int) []int {
	seen := make([]bool, max-min+1)
	var result []int
	for _, v := range vals {
		idx := v - min
		if idx >= 0 && idx < len(seen) && !seen[idx] {
			seen[idx] = true
			result = append(result, v)
		}
	}
	sorted := make([]int, 0, len(result))
	for i := range seen {
		if seen[i] {
			sorted = append(sorted, i+min)
		}
	}
	return sorted
}

func (cs *CronSchedule) Next(from time.Time) time.Time {
	t := from.Truncate(time.Minute).Add(time.Minute)

	for i := 0; i < 525960; i++ {
		if contains(cs.months, int(t.Month())) &&
			contains(cs.daysOfMonth, t.Day()) &&
			contains(cs.daysOfWeek, int(t.Weekday())) &&
			contains(cs.hours, t.Hour()) &&
			contains(cs.minutes, t.Minute()) {
			return t
		}
		t = t.Add(time.Minute)
	}

	return from.Add(24 * time.Hour)
}

func contains(vals []int, v int) bool {
	for _, val := range vals {
		if val == v {
			return true
		}
	}
	return false
}
