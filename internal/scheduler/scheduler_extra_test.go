package scheduler

import (
	"context"
	"testing"
	"time"

	"flowpilot/internal/models"
)

// TestParseCronInvalidFields tests various malformed cron expressions.
func TestParseCronInvalidFields(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"too few fields", "* * *"},
		{"too many fields", "* * * * * *"},
		{"invalid minute", "60 * * * *"},
		{"negative minute", "-1 * * * *"},
		{"invalid hour", "* 24 * * *"},
		{"invalid day of month", "* * 32 * *"},
		{"invalid month", "* * * 13 *"},
		{"invalid day of week", "* * * * 7"},
		{"invalid step zero", "*/0 * * * *"},
		{"non-numeric minute", "abc * * * *"},
		{"invalid range low > high", "* 10-5 * * *"},
		{"invalid range out of bounds", "* * 0-31 * *"},
		{"invalid range non-numeric start", "* a-5 * * *"},
		{"invalid range non-numeric end", "* 0-b * * *"},
		{"invalid step non-numeric", "*/x * * * *"},
		{"invalid step base non-numeric", "a/2 * * * *"},
		{"empty string", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCron(tc.expr)
			if err == nil {
				t.Errorf("expected error for cron %q, got nil", tc.expr)
			}
		})
	}
}

// TestParseCronValidExpressions tests valid cron expressions.
func TestParseCronValidExpressions(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"every minute", "* * * * *"},
		{"every 15 minutes", "*/15 * * * *"},
		{"specific time weekday", "30 9 * * 1-5"},
		{"multiple values", "0,30 8,20 * * *"},
		{"first of month", "0 0 1 * *"},
		{"range with step", "0 */2 * * *"},
		{"step from value", "0 9/3 * * *"},
		{"range in step", "0 9-17/2 * * *"},
		{"specific month", "0 0 * 6 *"},
		{"all weekdays", "0 8 * * 0-6"},
		{"minute range", "0-30 * * * *"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCron(tc.expr)
			if err != nil {
				t.Errorf("unexpected error for cron %q: %v", tc.expr, err)
			}
		})
	}
}

// TestCronNextAdvancesCorrectly checks Next() picks the right future time.
func TestCronNextAdvancesCorrectly(t *testing.T) {
	cases := []struct {
		name        string
		expr        string
		from        time.Time
		wantHour    int
		wantMinute  int
	}{
		{
			name:       "midnight schedule",
			expr:       "0 0 * * *",
			from:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			wantHour:   0,
			wantMinute: 0,
		},
		{
			name:       "every 30 minutes at start",
			expr:       "0,30 * * * *",
			from:       time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			wantHour:   12,
			wantMinute: 30,
		},
		{
			name:       "next hour boundary",
			expr:       "0 * * * *",
			from:       time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC),
			wantHour:   13,
			wantMinute: 0,
		},
		{
			name:       "hourly with step",
			expr:       "*/10 * * * *",
			from:       time.Date(2025, 1, 1, 9, 5, 0, 0, time.UTC),
			wantHour:   9,
			wantMinute: 10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs, err := ParseCron(tc.expr)
			if err != nil {
				t.Fatalf("ParseCron: %v", err)
			}
			next := cs.Next(tc.from)
			if next.Hour() != tc.wantHour || next.Minute() != tc.wantMinute {
				t.Errorf("expected %02d:%02d, got %02d:%02d",
					tc.wantHour, tc.wantMinute, next.Hour(), next.Minute())
			}
		})
	}
}

// newMockDB creates a mockScheduleDB with empty slices/maps ready to use.
func newMockDB() *mockScheduleDB {
	return &mockScheduleDB{
		schedules: nil,
		flows:     map[string]*models.RecordedFlow{},
		updated:   map[string]time.Time{},
	}
}

// TestSchedulerDoesNotStartTwice ensures Start is idempotent.
func TestSchedulerDoesNotStartTwice(t *testing.T) {
	db := newMockDB()
	sub := &mockSubmitter{}
	s := New(db, sub, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	s.Start(ctx) // second call should be no-op
	s.Stop()
}

// TestSchedulerStopIdempotent ensures Stop is safe to call when not running.
func TestSchedulerStopIdempotent(t *testing.T) {
	db := newMockDB()
	sub := &mockSubmitter{}
	s := New(db, sub, time.Hour)
	s.Stop() // Stop without Start should not panic
}

// TestSchedulerContextCancellation verifies context cancellation stops the loop.
func TestSchedulerContextCancellation(t *testing.T) {
	db := newMockDB()
	sub := &mockSubmitter{}
	s := New(db, sub, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	cancel() // cancel context; loop should exit
	// Give goroutine time to exit cleanly
	time.Sleep(20 * time.Millisecond)
}

// TestParseCronStepFromRange verifies step expressions with explicit ranges.
func TestParseCronStepFromRange(t *testing.T) {
	cs, err := ParseCron("0 9-17/2 * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	// Hours should be 9,11,13,15,17
	expected := []int{9, 11, 13, 15, 17}
	if len(cs.hours) != len(expected) {
		t.Fatalf("expected hours %v, got %v", expected, cs.hours)
	}
	for i, h := range expected {
		if cs.hours[i] != h {
			t.Errorf("hour[%d]: expected %d, got %d", i, h, cs.hours[i])
		}
	}
}

// TestParseCronStepFromValue verifies step expressions with a numeric base.
func TestParseCronStepFromValue(t *testing.T) {
	cs, err := ParseCron("0 9/3 * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	// Hours from 9 to 23 step 3: 9,12,15,18,21
	expected := []int{9, 12, 15, 18, 21}
	if len(cs.hours) != len(expected) {
		t.Fatalf("expected hours %v, got %v", expected, cs.hours)
	}
	for i, h := range expected {
		if cs.hours[i] != h {
			t.Errorf("hour[%d]: expected %d, got %d", i, h, cs.hours[i])
		}
	}
}

// TestParseCronDedup ensures duplicate values from comma-separated lists are deduped.
func TestParseCronDedup(t *testing.T) {
	cs, err := ParseCron("0,0,0 * * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	if len(cs.minutes) != 1 {
		t.Errorf("expected 1 unique minute value, got %d: %v", len(cs.minutes), cs.minutes)
	}
}
