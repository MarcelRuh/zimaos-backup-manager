package static

import (
	"embed"
	"io/fs"
	"testing/fstest"
)

//go:embed all:dist
var FS embed.FS

// Empty is used when dist is unavailable in tests.
var Empty fs.FS = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><html><body><p>ZimaOS Backup Manager UI not built. Run frontend build.</p></body></html>`)},
}
