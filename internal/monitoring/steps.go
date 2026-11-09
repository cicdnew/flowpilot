package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var StepDurationBucketsMs = []int64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

const (
	DefaultSlowStepThreshold = 10 * time.Second
	maxSlowSteps             = 100
)

type ActionStepStats struct {
	Action     string  `json:"action"`
	Count      int64   `json:"count"`
	Errors     int64   `json:"errors"`
	ErrorRate  float64 `json:"errorRate"`
	AvgMs      float64 `json:"avgMs"`
	MaxMs      int64   `json:"maxMs"`
	P50Ms      int64   `json:"p50Ms"`
	P95Ms      int64   `json:"p95Ms"`
	SlowCount  int64   `json:"slowCount"`
	LastSeenAt string  `json:"lastSeenAt,omitempty"`
}

type SlowStep struct {
	TaskID     string    `json:"taskId"`
	StepIndex  int       `json:"stepIndex"`
	Action     string    `json:"action"`
	DurationMs int64     `json:"durationMs"`
	Error      string    `json:"error,omitempty"`
	At         time.Time `json:"at"`
}

type actionHistogram struct {
	buckets []int64
	count   int64
	errors  int64
	sumMs   int64
	maxMs   int64
	slow    int64
	last    time.Time
}

type StepTracker struct {
	mu        sync.RWMutex
	threshold time.Duration
	actions   map[string]*actionHistogram
	slow      []SlowStep
	slowNext  int
}

func NewStepTracker(threshold time.Duration) *StepTracker {
	if threshold <= 0 {
		threshold = DefaultSlowStepThreshold
	}
	return &StepTracker{threshold: threshold, actions: map[string]*actionHistogram{}}
}

func (s *StepTracker) SetSlowThreshold(d time.Duration) {
	if d <= 0 {
		return
	}
	s.mu.Lock()
	s.threshold = d
	s.mu.Unlock()
}

func (s *StepTracker) Record(taskID, action string, stepIndex int, duration time.Duration, stepErr error) {
	ms := duration.Milliseconds()
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.actions[action]
	if !ok {
		h = &actionHistogram{buckets: make([]int64, len(StepDurationBucketsMs)+1)}
		s.actions[action] = h
	}
	idx := sort.Search(len(StepDurationBucketsMs), func(i int) bool { return ms <= StepDurationBucketsMs[i] })
	h.buckets[idx]++
	h.count++
	h.sumMs += ms
	h.maxMs = max(h.maxMs, ms)
	h.last = time.Now()
	if stepErr != nil {
		h.errors++
	}
	if duration >= s.threshold {
		h.slow++
		entry := SlowStep{TaskID: taskID, StepIndex: stepIndex, Action: action, DurationMs: ms, At: h.last}
		if stepErr != nil {
			entry.Error = stepErr.Error()
		}
		if len(s.slow) < maxSlowSteps {
			s.slow = append(s.slow, entry)
		} else {
			s.slow[s.slowNext] = entry
		}
		s.slowNext = (s.slowNext + 1) % maxSlowSteps
	}
}

func quantileMs(h *actionHistogram, q float64) int64 {
	if h.count == 0 {
		return 0
	}
	target := int64(float64(h.count)*q + 0.999999)
	var seen int64
	for i, n := range h.buckets {
		seen += n
		if seen >= target {
			if i < len(StepDurationBucketsMs) {
				return min(StepDurationBucketsMs[i], h.maxMs)
			}
			return h.maxMs
		}
	}
	return h.maxMs
}

func (s *StepTracker) Stats() []ActionStepStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ActionStepStats, 0, len(s.actions))
	for action, h := range s.actions {
		st := ActionStepStats{
			Action: action, Count: h.count, Errors: h.errors, MaxMs: h.maxMs, SlowCount: h.slow,
			P50Ms: quantileMs(h, 0.50), P95Ms: quantileMs(h, 0.95),
		}
		if h.count > 0 {
			st.AvgMs = float64(h.sumMs) / float64(h.count)
			st.ErrorRate = float64(h.errors) / float64(h.count)
			st.LastSeenAt = h.last.UTC().Format(time.RFC3339)
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Action < out[j].Action })
	return out
}

func (s *StepTracker) SlowSteps(limit int) []SlowStep {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.slow)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]SlowStep, 0, limit)
	for i := range limit {
		out = append(out, s.slow[(s.slowNext-1-i+2*n)%n])
	}
	return out
}

func escapeLabel(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}

func (s *StepTracker) Prometheus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	actions := make([]string, 0, len(s.actions))
	for a := range s.actions {
		actions = append(actions, a)
	}
	sort.Strings(actions)
	var b strings.Builder
	b.WriteString("# HELP flowpilot_step_duration_ms Step execution duration in milliseconds by action.\n")
	b.WriteString("# TYPE flowpilot_step_duration_ms histogram\n")
	for _, a := range actions {
		h := s.actions[a]
		label := escapeLabel(a)
		var cumulative int64
		for i, le := range StepDurationBucketsMs {
			cumulative += h.buckets[i]
			fmt.Fprintf(&b, "flowpilot_step_duration_ms_bucket{action=\"%s\",le=\"%d\"} %d\n", label, le, cumulative)
		}
		fmt.Fprintf(&b, "flowpilot_step_duration_ms_bucket{action=\"%s\",le=\"+Inf\"} %d\n", label, h.count)
		fmt.Fprintf(&b, "flowpilot_step_duration_ms_sum{action=\"%s\"} %d\n", label, h.sumMs)
		fmt.Fprintf(&b, "flowpilot_step_duration_ms_count{action=\"%s\"} %d\n", label, h.count)
	}
	b.WriteString("# HELP flowpilot_step_errors_total Failed step executions by action.\n")
	b.WriteString("# TYPE flowpilot_step_errors_total counter\n")
	for _, a := range actions {
		fmt.Fprintf(&b, "flowpilot_step_errors_total{action=\"%s\"} %d\n", escapeLabel(a), s.actions[a].errors)
	}
	b.WriteString("# HELP flowpilot_step_slow_total Steps slower than the slow-step threshold by action.\n")
	b.WriteString("# TYPE flowpilot_step_slow_total counter\n")
	for _, a := range actions {
		fmt.Fprintf(&b, "flowpilot_step_slow_total{action=\"%s\"} %d\n", escapeLabel(a), s.actions[a].slow)
	}
	return b.String()
}
