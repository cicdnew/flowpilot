package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func killProcess(t *testing.T, p *Process) {
	t.Helper()
	p.mu.Lock()
	proc := p.cmd.Process
	exited := p.exited
	p.mu.Unlock()
	if err := proc.Kill(); err != nil {
		t.Fatal(err)
	}
	<-exited
}

func TestProcessPluginRestartsAfterExternalKill(t *testing.T) {
	reg, p := loadFake(t, "ok")
	res, err := execFake(reg, "fake_ok_action", "before")
	if err != nil {
		t.Fatal(err)
	}
	oldPID := res.Variables["pid"]
	killProcess(t, p)

	res, err = execFake(reg, "fake_ok_action", "after")
	if err != nil {
		t.Fatalf("call after crash should transparently restart the plugin: %v", err)
	}
	if res.Variables["pid"] == oldPID || res.Variables["echo"] != "after" {
		t.Fatalf("expected a fresh process: old pid %s, result %+v", oldPID, res.Variables)
	}
	st := p.Status()
	if !st.Running || st.Restarts != 1 || !strings.Contains(st.LastError, "exited") {
		t.Fatalf("status after restart: %+v", st)
	}
	if info := reg.List(); info[0].Status == nil || info[0].Status.Restarts != 1 {
		t.Fatalf("registry info should expose runtime status: %+v", info[0].Status)
	}
}

func TestProcessPluginCrashLoopIsBackedOffAndLimited(t *testing.T) {
	reg, p := loadFake(t, "crash", `,"max_restarts":2`)
	if _, err := execFake(reg, "fake_crash_action", "x"); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("first crash: %v", err)
	}
	if _, err := execFake(reg, "fake_crash_action", "x"); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("first restart should run and crash again: %v", err)
	}
	start := time.Now()
	_, err := execFake(reg, "fake_crash_action", "x")
	if err == nil || !strings.Contains(err.Error(), "restart in") || time.Since(start) > 50*time.Millisecond {
		t.Fatalf("restart inside the backoff window must fail fast: %v (%s)", err, time.Since(start))
	}
	time.Sleep(initialRestartDelay + 50*time.Millisecond)
	if _, err := execFake(reg, "fake_crash_action", "x"); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("second restart after backoff: %v", err)
	}
	time.Sleep(2*initialRestartDelay + 50*time.Millisecond)
	_, err = execFake(reg, "fake_crash_action", "x")
	if err == nil || !strings.Contains(err.Error(), "restart limit of 2") {
		t.Fatalf("restart limit not enforced: %v", err)
	}
	if st := p.Status(); st.Running || st.Restarts != 2 {
		t.Fatalf("status after crash loop: %+v", st)
	}
}

func TestProcessPluginRestartDisabled(t *testing.T) {
	reg, p := loadFake(t, "ok", `,"max_restarts":-1`)
	killProcess(t, p)
	if _, err := execFake(reg, "fake_ok_action", "x"); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("restart should be disabled: %v", err)
	}
	if st := p.Status(); st.Restarts != 0 || st.Running {
		t.Fatalf("status: %+v", st)
	}
}

func TestProcessPluginNotRestartedAfterShutdown(t *testing.T) {
	reg, p := loadFake(t, "ok")
	if err := reg.Unload(context.Background(), "fake-ok"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Execute(context.Background(), StepContext{}); err == nil {
		t.Fatal("a shut down plugin must not be restarted")
	}
	if st := p.Status(); st.Running || st.Restarts != 0 {
		t.Fatalf("status: %+v", st)
	}
}
