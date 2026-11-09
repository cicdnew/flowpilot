package monitoring

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStepTrackerStatsAndQuantiles(t *testing.T) {
	s := NewStepTracker(time.Second)
	for i := 0; i < 90; i++ {
		s.Record("t1", "click", i, 40*time.Millisecond, nil)
	}
	for i := 0; i < 10; i++ {
		s.Record("t1", "click", i, 2*time.Second, errors.New("timeout"))
	}
	s.Record("t2", "navigate", 0, 300*time.Millisecond, nil)

	stats := s.Stats()
	if len(stats) != 2 || stats[0].Action != "click" || stats[1].Action != "navigate" {
		t.Fatalf("stats should be sorted per action: %+v", stats)
	}
	c := stats[0]
	if c.Count != 100 || c.Errors != 10 || c.SlowCount != 10 || c.ErrorRate != 0.1 {
		t.Fatalf("click counts: %+v", c)
	}
	if c.P50Ms != 50 || c.P95Ms != 2000 || c.MaxMs != 2000 {
		t.Fatalf("click quantiles: p50=%d p95=%d max=%d", c.P50Ms, c.P95Ms, c.MaxMs)
	}
	if want := (90*40.0 + 10*2000.0) / 100; c.AvgMs != want {
		t.Fatalf("avg: got %v want %v", c.AvgMs, want)
	}
	if n := stats[1]; n.P50Ms != 300 || n.P95Ms != 300 {
		t.Fatalf("single-sample quantiles should clamp to the observed max: %+v", n)
	}
}

func TestStepTrackerSlowStepsNewestFirstAndBounded(t *testing.T) {
	s := NewStepTracker(100 * time.Millisecond)
	s.Record("fast", "click", 0, 10*time.Millisecond, nil)
	for i := 0; i < maxSlowSteps+25; i++ {
		s.Record(fmt.Sprintf("task-%d", i), "wait", i, 200*time.Millisecond, nil)
	}
	all := s.SlowSteps(0)
	if len(all) != maxSlowSteps {
		t.Fatalf("slow buffer should be bounded to %d, got %d", maxSlowSteps, len(all))
	}
	if all[0].TaskID != fmt.Sprintf("task-%d", maxSlowSteps+24) || all[len(all)-1].TaskID != "task-25" {
		t.Fatalf("slow steps not newest-first: first=%s last=%s", all[0].TaskID, all[len(all)-1].TaskID)
	}
	top := s.SlowSteps(3)
	if len(top) != 3 || top[2].StepIndex != maxSlowSteps+22 {
		t.Fatalf("limited slow steps: %+v", top)
	}

	partial := NewStepTracker(100 * time.Millisecond)
	partial.Record("a", "wait", 0, time.Second, errors.New("boom"))
	partial.Record("b", "wait", 1, time.Second, nil)
	got := partial.SlowSteps(10)
	if len(got) != 2 || got[0].TaskID != "b" || got[1].Error != "boom" {
		t.Fatalf("partial buffer: %+v", got)
	}
}

func TestStepTrackerPrometheusHistogram(t *testing.T) {
	s := NewStepTracker(time.Second)
	s.Record("t", `we"ird`, 0, 5*time.Millisecond, nil)
	s.Record("t", "click", 0, 75*time.Millisecond, nil)
	s.Record("t", "click", 1, 75*time.Second, errors.New("x"))
	out := s.Prometheus()
	for _, want := range []string{
		"# TYPE flowpilot_step_duration_ms histogram",
		`flowpilot_step_duration_ms_bucket{action="click",le="50"} 0`,
		`flowpilot_step_duration_ms_bucket{action="click",le="100"} 1`,
		`flowpilot_step_duration_ms_bucket{action="click",le="60000"} 1`,
		`flowpilot_step_duration_ms_bucket{action="click",le="+Inf"} 2`,
		`flowpilot_step_duration_ms_count{action="click"} 2`,
		`flowpilot_step_errors_total{action="click"} 1`,
		`flowpilot_step_slow_total{action="click"} 1`,
		`action="we\"ird"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prometheus output missing %q\n%s", want, out)
		}
	}
}

func TestStepTrackerConcurrentRecording(t *testing.T) {
	s := NewStepTracker(time.Millisecond)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				s.Record("t", fmt.Sprintf("a%d", g%2), i, 2*time.Millisecond, nil)
				_ = s.Stats()
				_ = s.SlowSteps(5)
			}
		}(g)
	}
	wg.Wait()
	var total int64
	for _, st := range s.Stats() {
		total += st.Count
	}
	if total != 4000 {
		t.Fatalf("lost samples under concurrency: %d", total)
	}
}

func TestMonitorRecordStepFeedsLegacyAverageAndPrometheus(t *testing.T) {
	m := New()
	m.RecordStep("t", "navigate", 0, 100*time.Millisecond, nil)
	m.RecordStep("t", "navigate", 1, 300*time.Millisecond, nil)
	if avg := m.GetAvgStepDuration(); avg != 200 {
		t.Fatalf("legacy average: %v", avg)
	}
	if !strings.Contains(m.GetPrometheusMetrics(), `flowpilot_step_duration_ms_count{action="navigate"} 2`) {
		t.Fatal("monitor prometheus output missing step histogram")
	}
}
