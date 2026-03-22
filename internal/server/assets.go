package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var assets embed.FS

// GetAssetsFS returns the filesystem for the embedded frontend assets
func GetAssetsFS() http.FileSystem {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
