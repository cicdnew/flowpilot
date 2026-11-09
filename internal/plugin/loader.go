package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const ManifestFile = "plugin.json"

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s: %v", ErrInvalidManifest, path, err)
	}
	if m.Type == "" {
		m.Type = TypeProcess
	}
	if m.Type != TypeProcess {
		return Manifest{}, fmt.Errorf("%w: %s: only %q plugins can be loaded from disk", ErrInvalidManifest, path, TypeProcess)
	}
	return m, m.Validate()
}

func LoadDir(ctx context.Context, reg *Registry, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var loaded []string
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(dir, e.Name())
		manifestPath := filepath.Join(pluginDir, ManifestFile)
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		m, err := LoadManifest(manifestPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		p, err := NewProcess(m, pluginDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", manifestPath, err))
			continue
		}
		if err := reg.Register(ctx, p, pluginDir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", manifestPath, err))
			continue
		}
		loaded = append(loaded, m.ID)
	}
	return loaded, errors.Join(errs...)
}
