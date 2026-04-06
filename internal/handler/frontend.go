package handler

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func NewFrontendHandler(frontendFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		name := frontendPath(r.URL.Path)
		switch {
		case name == "index.html":
			serveFrontendFile(frontendFS, w, r, "index.html")
			return
		case fileExists(frontendFS, name):
			serveFrontendFile(frontendFS, w, r, name)
			return
		case fileExists(frontendFS, path.Join(name, "index.html")):
			serveFrontendFile(frontendFS, w, r, path.Join(name, "index.html"))
			return
		default:
			serveFrontendFile(frontendFS, w, r, "index.html")
		}
	})
}

func frontendPath(requestPath string) string {
	cleaned := path.Clean("/" + requestPath)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "index.html"
	}

	return cleaned
}

func fileExists(frontendFS fs.FS, name string) bool {
	info, err := fs.Stat(frontendFS, name)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func serveFrontendFile(frontendFS fs.FS, w http.ResponseWriter, r *http.Request, name string) {
	info, err := fs.Stat(frontendFS, name)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	content, err := fs.ReadFile(frontendFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, path.Base(name), info.ModTime(), bytes.NewReader(content))
}
