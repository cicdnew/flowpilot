package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"flowpilot/internal/plugin"
)

func installExamplePlugin(t *testing.T, pluginsDir, id string) {
	t.Helper()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("examples", "plugins", "text-tools")
	build := exec.Command(goBin, "build", "-o", filepath.Join(dir, "text-tools"), ".")
	build.Dir = src
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build example plugin: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(filepath.Join(src, plugin.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["id"] = id
	m["config"] = map[string]any{"blocked_hosts": []string{"secret-internal.example"}}
	m["actions"] = []map[string]any{{"name": strings.ReplaceAll(id, "-", "_") + "_transform", "value_required": true}}
	raw, _ = json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, plugin.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAppPluginLifecycle(t *testing.T) {
	app := &App{dataDir: t.TempDir(), plugins: plugin.NewRegistry()}
	installExamplePlugin(t, app.pluginsDir(), "text-tools")
	broken := filepath.Join(app.pluginsDir(), "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, plugin.ManifestFile), []byte(`{"id":"broken"`), 0o644); err != nil {
		t.Fatal(err)
	}

	app.initPlugins(t.Context())
	t.Cleanup(app.shutdownPlugins)
	status := app.ListPlugins()
	if status.Dir != app.pluginsDir() || len(status.Plugins) != 1 || status.Plugins[0].ID != "text-tools" {
		t.Fatalf("status after startup: %+v", status)
	}
	if status.Plugins[0].Config != nil {
		t.Fatal("plugin config (may hold secrets) must not be exposed to the UI")
	}
	if len(status.Errors) != 1 || !strings.Contains(status.Errors[0], "broken") {
		t.Fatalf("broken plugin not reported: %v", status.Errors)
	}

	installExamplePlugin(t, app.pluginsDir(), "text-tools-two")
	if err := os.RemoveAll(broken); err != nil {
		t.Fatal(err)
	}
	status, err := app.ReloadPlugins()
	if err != nil {
		t.Fatalf("ReloadPlugins: %v", err)
	}
	var ids []string
	for _, p := range status.Plugins {
		ids = append(ids, p.ID)
	}
	if strings.Join(ids, ",") != "text-tools,text-tools-two" || len(status.Errors) != 0 {
		t.Fatalf("after reload: ids=%v errors=%v", ids, status.Errors)
	}
	if _, ok := app.plugins.Action("text_tools_two_transform"); !ok {
		t.Fatal("reloaded plugin action not registered")
	}

	app.shutdownPlugins()
	if len(app.ListPlugins().Plugins) != 0 {
		t.Fatal("plugins still registered after shutdown")
	}
}

func TestAppPluginsMissingDirIsCreated(t *testing.T) {
	app := &App{dataDir: t.TempDir(), plugins: plugin.NewRegistry()}
	app.initPlugins(t.Context())
	if info, err := os.Stat(app.pluginsDir()); err != nil || !info.IsDir() {
		t.Fatalf("plugins dir not created: %v", err)
	}
	if status := app.ListPlugins(); len(status.Plugins) != 0 || len(status.Errors) != 0 {
		t.Fatalf("empty plugin dir status: %+v", status)
	}
}
