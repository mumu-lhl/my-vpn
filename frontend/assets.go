//go:build looma_embed_frontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var files embed.FS

// Assets exposes the built frontend at the root of the filesystem.
func Assets() fs.FS {
	assets, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return assets
}
