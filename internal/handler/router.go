package handler

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/notify"
	"rsync-backup-service/internal/store"
)

type Handler struct {
	db                *store.DB
	jwtSecret         string
	passwordSender    notify.PasswordSender
	passwordGenerator func() (string, error)
	loginLimiter      *loginRateLimiter
}

type RouterOption func(*routerOptions)

type routerOptions struct {
	frontend          http.Handler
	jwtSecret         string
	passwordSender    notify.PasswordSender
	passwordGenerator func() (string, error)
	loginLimiter      *loginRateLimiter
}

func WithFrontend(frontend http.Handler) RouterOption {
	return func(options *routerOptions) {
		options.frontend = frontend
	}
}

func WithJWTSecret(secret string) RouterOption {
	return func(options *routerOptions) {
		options.jwtSecret = secret
	}
}

func withPasswordSender(sender notify.PasswordSender) RouterOption {
	return func(options *routerOptions) {
		options.passwordSender = sender
	}
}

func withPasswordGenerator(generator func() (string, error)) RouterOption {
	return func(options *routerOptions) {
		options.passwordGenerator = generator
	}
}

func withLoginLimiter(limiter *loginRateLimiter) RouterOption {
	return func(options *routerOptions) {
		options.loginLimiter = limiter
	}
}

func NewRouter(db *store.DB, options ...RouterOption) http.Handler {
	resolved := routerOptions{}
	for _, option := range options {
		option(&resolved)
	}
	if resolved.passwordSender == nil {
		resolved.passwordSender = notify.NewPasswordSender()
	}
	if resolved.passwordGenerator == nil {
		resolved.passwordGenerator = generateRandomPassword
	}
	if resolved.loginLimiter == nil {
		resolved.loginLimiter = newLoginRateLimiter(time.Now)
	}

	handler := &Handler{
		db:                db,
		jwtSecret:         resolved.jwtSecret,
		passwordSender:    resolved.passwordSender,
		passwordGenerator: resolved.passwordGenerator,
		loginLimiter:      resolved.loginLimiter,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handler.Health)
	mux.HandleFunc("POST /api/v1/auth/register", handler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", handler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", handler.Refresh)
	authenticated := middleware.Auth(resolved.jwtSecret)
	mux.Handle("GET /api/v1/users", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListUsers))))
	mux.Handle("POST /api/v1/users", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreateUser))))
	mux.Handle("PUT /api/v1/users/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateUser))))
	mux.Handle("DELETE /api/v1/users/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeleteUser))))
	mux.Handle("GET /api/v1/users/me", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.GetCurrentUser))))
	mux.Handle("PUT /api/v1/users/me/password", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.UpdateCurrentUserPassword))))
	mux.Handle("PUT /api/v1/users/me/profile", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.UpdateCurrentUserProfile))))
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
