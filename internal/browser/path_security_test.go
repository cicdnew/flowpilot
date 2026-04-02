package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingDirWithinBase_AllowsDirectoryInsideBase(t *testing.T) {
	base := t.TempDir()
	child := filepath.Join(base, "downloads")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	resolvedBase, resolvedChild, err := resolveExistingDirWithinBase(base, child)
	if err != nil {
		t.Fatalf("resolveExistingDirWithinBase: %v", err)
	}
	if !pathWithinBase(resolvedBase, resolvedChild) {
		t.Fatalf("expected %q to be within %q", resolvedChild, resolvedBase)
	}
}

func TestResolveExistingDirWithinBase_RejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link-out")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, _, err := resolveExistingDirWithinBase(base, link)
	if err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
