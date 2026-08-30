//go:build !studio_embed

package web

import (
	"io/fs"
	"testing/fstest"
)

// Assets reports that Studio assets were not embedded in this build.
func Assets() (fs.FS, bool) {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><html><head><meta charset="utf-8"><title>Kick Sim Studio</title></head><body><main><h1>Studio assets are not embedded</h1><p>Build the frontend and compile with the studio_embed tag.</p></main></body></html>`)},
	}, false
}
