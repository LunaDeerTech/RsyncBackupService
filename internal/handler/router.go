package handler

import (
	"bytes"
	"net/http"
	"strings"

	"rsync-backup-service/internal/store"
)

type Handler struct {
	db *store.DB
}

type RouterOption func(*routerOptions)

type routerOptions struct {
	frontend http.Handler
}

func WithFrontend(frontend http.Handler) RouterOption {
	return func(options *routerOptions) {
		options.frontend = frontend
	}
}

func NewRouter(db *store.DB, options ...RouterOption) http.Handler {
	handler := &Handler{db: db}
	resolved := routerOptions{}
	for _, option := range options {
		option(&resolved)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handler.Health)
	if resolved.frontend != nil {
		mux.Handle("/", resolved.frontend)
	}

	return withAPIErrors(mux)
}

func withAPIErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		recorder := newBufferedResponseWriter()
		next.ServeHTTP(recorder, r)

		switch recorder.statusCode() {
		case http.StatusNotFound:
			Error(w, http.StatusNotFound, 40401, "resource not found")
			return
		case http.StatusMethodNotAllowed:
			copyHeaders(w.Header(), recorder.Header())
			Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
			return
		}

		copyHeaders(w.Header(), recorder.Header())
		w.WriteHeader(recorder.statusCode())
		_, _ = w.Write(recorder.body.Bytes())
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

type bufferedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{header: make(http.Header)}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.body.Write(data)
}

func (w *bufferedResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *bufferedResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}

	return w.status
}
