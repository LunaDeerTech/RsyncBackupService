package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rsync-backup-service/internal/handler"
	"rsync-backup-service/internal/middleware"
)

func TestServerMiddlewareChainDoesNotDuplicateCORSHeadersOnUnauthorizedAPIResponse(t *testing.T) {
	router := middleware.CSRFProtection(handler.NewRouter(nil, handler.WithJWTSecret("secret")))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/3/stats", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertSingleHeaderValue(t, recorder.Header(), "Access-Control-Allow-Origin", "*")
	assertSingleHeaderValue(t, recorder.Header(), "Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	assertSingleHeaderValue(t, recorder.Header(), "Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
	assertSingleHeaderValue(t, recorder.Header(), "Access-Control-Max-Age", "600")
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}
}

func assertSingleHeaderValue(t *testing.T, header http.Header, key, want string) {
	t.Helper()

	values := header.Values(key)
	if len(values) != 1 {
		t.Fatalf("%s values = %q, want exactly one value %q", key, values, want)
	}
	if values[0] != want {
		t.Fatalf("%s = %q, want %q", key, values[0], want)
	}
}