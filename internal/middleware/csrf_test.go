package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFAllowsGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called for GET")
	}
}

func TestCSRFAllowsJSONContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called for JSON POST")
	}
}

func TestCSRFAllowsXRequestedWith(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called with X-Requested-With")
	}
}

func TestCSRFBlocksPlainPOST(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rec, req)
	if called {
		t.Fatal("handler was called for plain POST without CSRF header")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRFAllowsDownloadPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/download/abc123", nil)
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called for download path")
	}
}

func TestCSRFAllowsMultipartFormData(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/remotes", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=---abc")
	rec := httptest.NewRecorder()
	called := false
	CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called for multipart request")
	}
}
