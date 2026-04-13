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

func TestNewRouterServesOpenAPIDocument(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v2/openapi.json", nil)
	req.Host = "example.com:8080"
	recorder := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	var document map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode openapi document: %v", err)
	}
	if document["openapi"] != "3.0.3" {
		t.Fatalf("openapi version = %v, want %q", document["openapi"], "3.0.3")
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths type = %T, want map[string]any", document["paths"])
	}
	if _, exists := paths["/api/v2/instances"]; !exists {
		t.Fatal("openapi document missing /api/v2/instances path")
	}
	if _, exists := paths["/api/v1/users/me/api-keys"]; exists {
		t.Fatal("openapi document unexpectedly exposes api key management paths")
	}
	if _, exists := paths["/api/v2/instances/{id}/plan"]; !exists {
		t.Fatal("openapi document missing /api/v2/instances/{id}/plan path")
	}
	if _, exists := paths["/api/v2/instances/{id}/disaster-recovery"]; !exists {
		t.Fatal("openapi document missing /api/v2/instances/{id}/disaster-recovery path")
	}
	components, ok := document["components"].(map[string]any)
	if !ok {
		t.Fatalf("components type = %T, want map[string]any", document["components"])
	}
	securitySchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatalf("securitySchemes type = %T, want map[string]any", components["securitySchemes"])
	}
	apiKeyScheme, ok := securitySchemes["ApiKeyAuth"].(map[string]any)
	if !ok {
		t.Fatalf("ApiKeyAuth scheme type = %T, want map[string]any", securitySchemes["ApiKeyAuth"])
	}
	if apiKeyScheme["type"] != "http" || apiKeyScheme["scheme"] != "bearer" {
		t.Fatalf("ApiKeyAuth scheme = %#v, want http bearer", apiKeyScheme)
	}
	servers, ok := document["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("servers = %#v, want one server entry", document["servers"])
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		t.Fatalf("server entry type = %T, want map[string]any", servers[0])
	}
	if server["url"] != "http://example.com:8080" {
		t.Fatalf("server url = %v, want %q", server["url"], "http://example.com:8080")
	}
}

func TestNewRouterHandlesAPIPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v2/instances", nil)
	recorder := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "*")
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
