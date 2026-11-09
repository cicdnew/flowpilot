package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "flowpilot/sdk/go/plugin"
)

const fakePluginEnv = "FLOWPILOT_FAKE_PLUGIN_MODE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakePluginEnv); mode != "" {
		runFakePlugin(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFakePlugin(mode string) {
	if mode == "badinit" {
		fmt.Fprintln(os.Stderr, "missing api key")
	}
	sc := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		var req sdk.Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			os.Exit(3)
		}
		resp := sdk.Response{ID: req.ID}
		switch req.Method {
		case sdk.MethodInit:
			if mode == "badinit" {
				resp.Error = "missing api key"
			}
		case sdk.MethodExecute:
			var sc sdk.StepContext
			_ = json.Unmarshal(req.Params, &sc)
			switch mode {
			case "hang":
				select {}
			case "crash":
				fmt.Fprintln(os.Stderr, "fatal: segfault in plugin")
				os.Exit(2)
			case "garbage":
				fmt.Fprintln(os.Stdout, "not json")
				continue
			}
			if sc.Step.Value == "fail" {
				resp.Error = "requested failure"
				break
			}
			res := sdk.StepResult{Variables: map[string]string{"echo": sc.Step.Value, "pid": fmt.Sprint(os.Getpid()), "env_secret": os.Getenv("FLOWPILOT_TEST_SECRET")}}
			resp.Result, _ = json.Marshal(res)
		case sdk.MethodHook:
			var ev sdk.HookEvent
			_ = json.Unmarshal(req.Params, &ev)
			if ev.Point == sdk.HookPreExecute && ev.Context.Step.Value == "veto" {
				resp.Error = "value not allowed"
			}
		case sdk.MethodShutdown:
			_ = out.Encode(resp)
			if mode == "slowexit" {
				time.Sleep(time.Hour)
			}
			return
		}
		_ = out.Encode(resp)
	}
}

func writeFakePlugin(t *testing.T, root, id, mode string, extra ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\n%s=%s exec %q -test.run=^$\n", fakePluginEnv, mode, exe)
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"id":%q,"name":"Fake %s","version":"0.1.0","entrypoint":"run.sh","timeout_ms":1500,
"actions":[{"name":%q,"value_required":true}],"hooks":["pre_execute"],"capabilities":["variables.write"]%s}`, id, mode, strings.ReplaceAll(id, "-", "_")+"_action", strings.Join(extra, ""))
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func loadFake(t *testing.T, mode string, extra ...string) (*Registry, *Process) {
	t.Helper()
	root := t.TempDir()
	writeFakePlugin(t, root, "fake-"+mode, mode, extra...)
	reg := NewRegistry()
	ids, err := LoadDir(context.Background(), reg, root)
	if err != nil || len(ids) != 1 {
		t.Fatalf("LoadDir: ids=%v err=%v", ids, err)
	}
	t.Cleanup(func() { _ = reg.Close(context.Background()) })
	reg.mu.RLock()
	p := reg.entries[ids[0]].plugin.(*Process)
	reg.mu.RUnlock()
	return reg, p
}

func execFake(reg *Registry, action, value string) (StepResult, error) {
	res, _, err := reg.Execute(context.Background(), StepContext{TaskID: "t", Step: sdk.Step{Action: action, Value: value}}, nil)
	return res, err
}

func TestProcessPluginLifecycleAndExecute(t *testing.T) {
	t.Setenv("FLOWPILOT_TEST_SECRET", "should-not-leak")
	reg, p := loadFake(t, "ok")
	if !p.Running() {
		t.Fatal("plugin process not running after load")
	}
	if info := reg.List(); len(info) != 1 || info[0].Type != TypeProcess || !strings.HasSuffix(info[0].Source, "fake-ok") {
		t.Fatalf("List: %+v", info)
	}
	res, err := execFake(reg, "fake_ok_action", "hello")
	if err != nil || res.Variables["echo"] != "hello" {
		t.Fatalf("Execute: res=%+v err=%v", res, err)
	}
	if res.Variables["env_secret"] != "" {
		t.Fatal("host environment leaked into plugin process")
	}
	pid := res.Variables["pid"]
	res, err = execFake(reg, "fake_ok_action", "again")
	if err != nil || res.Variables["pid"] != pid {
		t.Fatalf("plugin process should be reused across calls: %v %v", res.Variables["pid"], err)
	}
	if _, err := execFake(reg, "fake_ok_action", "fail"); err == nil || !strings.Contains(err.Error(), "requested failure") {
		t.Fatalf("plugin-reported error lost: %v", err)
	}
	if !p.Running() {
		t.Fatal("a plugin-reported error must not kill the process")
	}
	if err := reg.RunHooks(context.Background(), HookPreExecute, StepContext{Step: sdk.Step{Value: "veto"}}, nil, nil); !errors.Is(err, ErrVetoed) {
		t.Fatalf("process hook veto: %v", err)
	}
	if err := reg.Unload(context.Background(), "fake-ok"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if p.Running() {
		t.Fatal("process still running after unload")
	}
}

func TestProcessPluginTimeoutKillsHungProcess(t *testing.T) {
	reg, p := loadFake(t, "hang")
	start := time.Now()
	_, err := execFake(reg, "fake_hang_action", "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timeout not enforced, took %s", elapsed)
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.Running() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if p.Running() {
		t.Fatal("hung plugin process was not killed")
	}
	if _, err := execFake(reg, "fake_hang_action", "x"); err == nil {
		t.Fatal("calls to a killed plugin must fail fast")
	}
}

func TestProcessPluginCrashIsIsolated(t *testing.T) {
	reg, _ := loadFake(t, "crash")
	_, err := execFake(reg, "fake_crash_action", "x")
	if !errors.Is(err, ErrProcessExited) || !strings.Contains(err.Error(), "segfault in plugin") {
		t.Fatalf("expected process-exited error with stderr tail, got %v", err)
	}
	if _, err := execFake(reg, "fake_crash_action", "x"); !errors.Is(err, ErrProcessExited) {
		t.Fatalf("subsequent call: %v", err)
	}
}

func TestProcessPluginProtocolErrorKillsProcess(t *testing.T) {
	reg, p := loadFake(t, "garbage")
	if _, err := execFake(reg, "fake_garbage_action", "x"); err == nil || !strings.Contains(err.Error(), "protocol error") {
		t.Fatalf("expected protocol error, got %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if p.Running() {
		t.Fatal("plugin speaking an invalid protocol must be stopped")
	}
}

func TestProcessPluginShutdownForceKillsSlowExit(t *testing.T) {
	reg, p := loadFake(t, "slowexit")
	start := time.Now()
	if err := reg.Unload(context.Background(), "fake-slowexit"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if elapsed := time.Since(start); elapsed > shutdownGrace+3*time.Second {
		t.Fatalf("shutdown took %s", elapsed)
	}
	if p.Running() {
		t.Fatal("slow plugin still running after shutdown")
	}
}

func TestLoadDirReportsBadPluginsAndLoadsGoodOnes(t *testing.T) {
	root := t.TempDir()
	writeFakePlugin(t, root, "a-good", "ok")
	writeFakePlugin(t, root, "b-badinit", "badinit")
	escape := filepath.Join(root, "c-escape")
	if err := os.MkdirAll(escape, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(escape, ManifestFile), []byte(`{"id":"c-escape","name":"x","version":"1","entrypoint":"../a-good/run.sh","actions":[{"name":"escape_action"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	typo := filepath.Join(root, "d-typo")
	if err := os.MkdirAll(typo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(typo, ManifestFile), []byte(`{"id":"d-typo","name":"x","version":"1","entrypoint":"run.sh","actoins":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	defer reg.Close(context.Background())
	ids, err := LoadDir(context.Background(), reg, root)
	if strings.Join(ids, ",") != "a-good" {
		t.Fatalf("loaded ids: %v", ids)
	}
	msg := fmt.Sprint(err)
	for _, want := range []string{"missing api key", "escapes the plugin directory", "unknown field"} {
		if !strings.Contains(msg, want) {
			t.Errorf("LoadDir error missing %q: %v", want, msg)
		}
	}
	if _, err := LoadDir(context.Background(), reg, filepath.Join(root, "missing")); err != nil {
		t.Fatalf("missing plugin dir should be a no-op: %v", err)
	}
}
