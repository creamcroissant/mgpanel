package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed all:static
var staticEmbed embed.FS

// getFileSystem returns the appropriate filesystem based on config and embedded assets
func getFileSystem(dir string) (http.FileSystem, error) {
	// First check if user provided an external directory
	if dir != "" {
		if _, err := os.Stat(dir); err == nil {
			return http.Dir(dir), nil
		}
	}

	// Fallback to embedded assets
	sub, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}