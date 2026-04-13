package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
)

const apiKeyErrorNotFound = 40410

type apiKeyRequest struct {
	Name string `json:"name"`
}

type apiKeyResponse struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type apiKeyCreateResponse struct {
	APIKey apiKeyResponse `json:"api_key"`
	Key    string         `json:"key"`
}

func (h *Handler) ListCurrentUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	user := middleware.MustGetUser(r.Context())
	apiKeys, err := h.db.ListAPIKeysByUser(user.UserID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list api keys")
		return
	}

	items := make([]apiKeyResponse, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		items = append(items, toAPIKeyResponse(apiKey))
	}

	JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateCurrentUserAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request apiKeyRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	name, err := normalizeAPIKeyName(request.Name)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	rawKey, err := authcrypto.GenerateAPIKey()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to generate api key")
		return
	}
	hash, err := authcrypto.HashAPIKey(rawKey)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to hash api key")
		return
	}

	claims := middleware.MustGetUser(r.Context())
	apiKey := &model.APIKey{
		UserID:    claims.UserID,
		Name:      name,
		KeyPrefix: authcrypto.APIKeyDisplayPrefix(rawKey),
		KeyHash:   hash,
		Key:       rawKey,
	}
	if err := h.db.CreateAPIKey(apiKey); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to create api key")
		return
	}

	JSON(w, http.StatusCreated, apiKeyCreateResponse{
		APIKey: toAPIKeyResponse(*apiKey),
		Key:    rawKey,
	})
}

func (h *Handler) DeleteCurrentUserAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	apiKeyID, err := apiKeyIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid api key id")
		return
	}

	claims := middleware.MustGetUser(r.Context())
	if err := h.db.DeleteAPIKeyByIDAndUser(apiKeyID, claims.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, apiKeyErrorNotFound, "api key not found")
			return
		}
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to delete api key")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "api key deleted"})
}

func toAPIKeyResponse(apiKey model.APIKey) apiKeyResponse {
	response := apiKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		KeyPrefix: apiKey.KeyPrefix,
		CreatedAt: apiKey.CreatedAt.Format(timeRFC3339),
	}
	if apiKey.LastUsedAt != nil {
		formatted := apiKey.LastUsedAt.Format(timeRFC3339)
		response.LastUsedAt = &formatted
	}

	return response
}

func apiKeyIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("api key id is required")
	}

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse api key id %q: %w", rawID, err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("api key id must be positive")
	}

	return id, nil
}

func normalizeAPIKeyName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if len([]rune(name)) > 64 {
		return "", fmt.Errorf("name must be 64 characters or less")
	}

	return name, nil
}
