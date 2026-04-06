package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNewFrontendHandlerServesExistingAsset(t *testing.T) {
	frontend := NewFrontendHandler(testFrontendFS())
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	recorder := httptest.NewRecorder()

	frontend.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != "console.log('rbs');" {
		t.Fatalf("body = %q, want %q", body, "console.log('rbs');")
	}
}

func TestNewFrontendHandlerFallsBackToIndex(t *testing.T) {
	frontend := NewFrontendHandler(testFrontendFS())
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()

	frontend.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `<div id="app"></div>`) {
		t.Fatalf("body = %q, want HTML shell", body)
	}
}

func TestNewRouterServesFrontendOutsideAPI(t *testing.T) {
	router := NewRouter(nil, WithFrontend(NewFrontendHandler(testFrontendFS())))
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `<div id="app"></div>`) {
		t.Fatalf("body = %q, want HTML shell", body)
	}
}

func testFrontendFS() fs.FS {
	return fstest.MapFS{
		"index.html": {
			Data: []byte("<!doctype html><html><body><div id=\"app\"></div></body></html>"),
		},
		"assets/app.js": {
			Data: []byte("console.log('rbs');"),
		},
	}
}
