package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h *Handler) OpenAPIDocument(w http.ResponseWriter, r *http.Request) {
	serverURL := openAPIServerURL(r)

	document := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Rsync Backup Service API",
			"version":     "1.0.0",
			"description": "API-key authenticated instance query endpoints.",
		},
		"servers": []map[string]any{{"url": serverURL}},
		"paths": map[string]any{
			"/api/v2/instances": map[string]any{
				"get": map[string]any{
					"summary":  "List accessible instances",
					"security": []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{
						openAPIQueryParameter("page", "integer"),
						openAPIQueryParameter("page_size", "integer"),
					},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/PaginatedInstances"),
					},
				},
			},
			"/api/v2/instances/{id}/overview": map[string]any{
				"get": map[string]any{
					"summary":    "Get instance overview",
					"security":   []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{openAPIPathParameter("id")},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/InstanceOverview"),
					},
				},
			},
			"/api/v2/instances/{id}/current-task": map[string]any{
				"get": map[string]any{
					"summary":    "Get current task for instance",
					"security":   []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{openAPIPathParameter("id")},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/CurrentTaskResponse"),
					},
				},
			},
			"/api/v2/instances/{id}/policies": map[string]any{
				"get": map[string]any{
					"summary":    "List instance policies",
					"security":   []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{openAPIPathParameter("id")},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/PolicyListResponse"),
					},
				},
			},
			"/api/v2/instances/{id}/plan": map[string]any{
				"get": map[string]any{
					"summary":  "List upcoming scheduled tasks for instance",
					"security": []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{
						openAPIPathParameter("id"),
						openAPIQueryParameter("within_hours", "integer"),
					},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/UpcomingTaskListResponse"),
					},
				},
			},
			"/api/v2/instances/{id}/disaster-recovery": map[string]any{
				"get": map[string]any{
					"summary":    "Get disaster recovery score details",
					"security":   []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{openAPIPathParameter("id")},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/DisasterRecoveryScore"),
					},
				},
			},
			"/api/v2/instances/{id}/backups": map[string]any{
				"get": map[string]any{
					"summary":  "List instance backups",
					"security": []map[string]any{{"ApiKeyAuth": []any{}}},
					"parameters": []map[string]any{
						openAPIPathParameter("id"),
						openAPIQueryParameter("page", "integer"),
						openAPIQueryParameter("page_size", "integer"),
					},
					"responses": map[string]any{
						"200": openAPIEnvelopeResponse("#/components/schemas/PaginatedBackups"),
					},
				},
			},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"ApiKeyAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "APIKey",
				},
			},
			"schemas": openAPISchemas(),
		},
	}

	payload, err := json.Marshal(document)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to encode openapi document")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func openAPIServerURL(r *http.Request) string {
	if r == nil {
		return "/"
	}

	scheme := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return "/"
	}

	return scheme + "://" + host
}

func openAPIEnvelopeResponse(schemaRef string) map[string]any {
	return map[string]any{
		"description": "Success",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "integer", "example": 0},
						"message": map[string]any{"type": "string", "example": "ok"},
						"data":    map[string]any{"$ref": schemaRef},
					},
				},
			},
		},
	}
}

func openAPIPathParameter(name string) map[string]any {
	return map[string]any{
		"name":     name,
		"in":       "path",
		"required": true,
		"schema":   map[string]any{"type": "integer"},
	}
}

func openAPIQueryParameter(name, typ string) map[string]any {
	return map[string]any{
		"name":   name,
		"in":     "query",
		"schema": map[string]any{"type": typ},
	}
}
