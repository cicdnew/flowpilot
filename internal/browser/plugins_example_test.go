package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"flowpilot/internal/models"
	"flowpilot/internal/plugin"
)

func buildTextToolsPlugin(t *testing.T) string {
	t.Helper()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "text-tools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("..", "..", "examples", "plugins", "text-tools")
	build := exec.Command(goBin, "build", "-o", filepath.Join(dir, "text-tools"), ".")
	build.Dir = src
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build example plugin: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(filepath.Join(src, plugin.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["config"] = map[string]any{"blocked_hosts": []string{"blocked.example"}}
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, plugin.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestExampleTextToolsPluginEndToEnd(t *testing.T) {
	root := buildTextToolsPlugin(t)
	reg := plugin.NewRegistry()
	t.Cleanup(func() { _ = reg.Close(context.Background()) })
	ids, err := plugin.LoadDir(context.Background(), reg, root)
	if err != nil || len(ids) != 1 || ids[0] != "text-tools" {
		t.Fatalf("LoadDir: %v %v", ids, err)
	}
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.SetPluginRegistry(reg)
	result := &models.TaskResult{TaskID: "e2e", ExtractedData: map[string]string{"var_product": "  Blue Widget Pro  "}}

	step := models.TaskStep{Action: "text_transform", Value: "Buy {{product}} now!", Operator: "slug", VarName: "slug"}
	if err := r.runPluginStep(context.Background(), 0, step, result); err != nil {
		t.Fatalf("text_transform: %v", err)
	}
	if got := result.ExtractedData["var_slug"]; got != "buy-blue-widget-pro-now" {
		t.Fatalf("slug: got %q", got)
	}
	if !strings.Contains(result.Logs[len(result.Logs)-1].Message, "[plugin text-tools] slug -> slug") {
		t.Fatalf("plugin log missing: %+v", result.Logs)
	}

	bad := models.TaskStep{Action: "text_transform", Value: "x", Operator: "rot13", VarName: "v"}
	if err := r.runPluginStep(context.Background(), 1, bad, result); err == nil || !strings.Contains(err.Error(), "unsupported operator") {
		t.Fatalf("expected operator error, got %v", err)
	}

	blocked := models.TaskStep{Action: models.ActionNavigate, Value: "https://shop.blocked.example/cart"}
	if err := r.runPluginStep(context.Background(), 2, blocked, result); !errors.Is(err, plugin.ErrVetoed) {
		t.Fatalf("expected navigation veto, got %v", err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("vetoed navigation reached the browser: %v", mock.calls)
	}
	allowed := models.TaskStep{Action: models.ActionNavigate, Value: "https://example.com"}
	if err := r.runPluginStep(context.Background(), 3, allowed, result); err != nil {
		t.Fatalf("allowed navigation: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("allowed navigation should reach the browser once: %v", mock.calls)
	}
}
