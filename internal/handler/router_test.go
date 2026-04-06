package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != 0 {
		t.Fatalf("code = %d, want %d", response.Code, 0)
	}
	if response.Message != "ok" {
		t.Fatalf("message = %q, want %q", response.Message, "ok")
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map[string]interface{}", response.Data)
	}
	if data["status"] != "healthy" {
		t.Fatalf("data.status = %v, want %q", data["status"], "healthy")
	}
}

func TestNewRouterReturnsJSONNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	recorder := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != 40401 {
		t.Fatalf("code = %d, want %d", response.Code, 40401)
	}
	if response.Message != "resource not found" {
		t.Fatalf("message = %q, want %q", response.Message, "resource not found")
	}
	if response.Data != nil {
		t.Fatalf("data = %v, want nil", response.Data)
	}
}
