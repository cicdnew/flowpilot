//go:build dev

package main

import "embed"

// Embed a file that is always present in the repo so the root package
// compiles without running the frontend build first. This is only used
// during development and CI testing (go test -tags=dev ./...).
//
//go:embed frontend/index.html
var assets embed.FS
