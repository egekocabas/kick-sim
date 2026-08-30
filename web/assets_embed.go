//go:build studio_embed

package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embedded embed.FS

// Assets returns the embedded Studio build and reports that it is available.
func Assets() (fs.FS, bool) {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return assets, true
}
