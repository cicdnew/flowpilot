//go:build !dev

package main

import (
	"embed"
	"io"
	"io/fs"
	"strings"
	"time"
)

// Embed the frontend tree so production builds can serve dist assets when they
// are present, while still allowing package loading in environments where the
// frontend build has not been run yet.
//
//go:embed frontend
var embeddedAssets embed.FS

var assets fs.FS = resolveAssetsFS()

func resolveAssetsFS() fs.FS {
	distAssets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err == nil {
		return distAssets
	}

	return staticAssetsFS{
		"index.html": []byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FlowPilot</title>
</head>
<body>
    <main style="font-family: sans-serif; max-width: 48rem; margin: 3rem auto; line-height: 1.5; padding: 0 1rem;">
        <h1>Frontend assets are not built</h1>
        <p>The application binary was started without a generated <code>frontend/dist</code> bundle.</p>
        <p>Run the frontend build (for example via <code>wails build</code> or <code>npm run build</code> in <code>frontend/</code>) and rebuild the app.</p>
    </main>
</body>
</html>`),
	}
}

type staticAssetsFS map[string][]byte

func (s staticAssetsFS) Open(name string) (fs.File, error) {
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		entries := make([]staticDirEntry, 0, len(s))
		for fileName, data := range s {
			entries = append(entries, staticDirEntry{name: fileName, size: int64(len(data))})
		}
		return &staticDirFile{entries: entries}, nil
	}

	data, ok := s[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	return &staticFile{
		Reader: strings.NewReader(string(data)),
		info: staticFileInfo{
			name: name,
			size: int64(len(data)),
		},
	}, nil
}

type staticFile struct {
	*strings.Reader
	info staticFileInfo
}

func (f *staticFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *staticFile) Close() error {
	return nil
}

type staticDirFile struct {
	entries []staticDirEntry
	offset  int
}

func (d *staticDirFile) Stat() (fs.FileInfo, error) {
	return staticFileInfo{name: ".", dir: true}, nil
}

func (d *staticDirFile) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (d *staticDirFile) Close() error {
	return nil
}

func (d *staticDirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.offset >= len(d.entries) && count > 0 {
		return nil, io.EOF
	}

	if count <= 0 || d.offset+count > len(d.entries) {
		count = len(d.entries) - d.offset
	}

	entries := make([]fs.DirEntry, 0, count)
	for _, entry := range d.entries[d.offset : d.offset+count] {
		entries = append(entries, entry)
	}
	d.offset += count

	return entries, nil
}

type staticDirEntry struct {
	name string
	size int64
}

func (e staticDirEntry) Name() string {
	return e.name
}

func (e staticDirEntry) IsDir() bool {
	return false
}

func (e staticDirEntry) Type() fs.FileMode {
	return 0
}

func (e staticDirEntry) Info() (fs.FileInfo, error) {
	return staticFileInfo{name: e.name, size: e.size}, nil
}

type staticFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i staticFileInfo) Name() string {
	return i.name
}

func (i staticFileInfo) Size() int64 {
	return i.size
}

func (i staticFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

func (i staticFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (i staticFileInfo) IsDir() bool {
	return i.dir
}

func (i staticFileInfo) Sys() any {
	return nil
}
