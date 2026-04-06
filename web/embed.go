package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var frontendFS embed.FS

func DistFS() (fs.FS, bool) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil, false
	}

	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		return nil, false
	}

	return distFS, true
}
