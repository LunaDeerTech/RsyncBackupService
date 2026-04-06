package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSHandlesPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()

	called := false
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, req)

	if called {
		t.Fatal("next handler was called for preflight request")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != allowMethods {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, allowMethods)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != allowHeaders {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %q", got, allowHeaders)
	}
}

func TestCORSPassesThroughNonPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
}
