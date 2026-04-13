package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
)

func TestAPIKeyAuthStoresUserClaimsInContext(t *testing.T) {
	db := newMiddlewareTestDB(t)
	user := createMiddlewareTestUser(t, db, "api-key-user@example.com", "viewer")
	rawKey := mustCreateMiddlewareAPIKey(t, db, user.ID, "viewer-key")

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instances", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	recorder := httptest.NewRecorder()

	handler := APIKeyAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := MustGetUser(r.Context())
		if claims.UserID != user.ID || claims.Email != user.Email || claims.Role != user.Role {
			t.Fatalf("claims = %+v, want user %+v", claims, user)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestAPIKeyAuthRejectsMissingKey(t *testing.T) {
	handler := APIKeyAuth(newMiddlewareTestDB(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instances", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
}

func mustCreateMiddlewareAPIKey(t *testing.T, db middlewareAPIKeyStore, userID int64, name string) string {
	t.Helper()

	rawKey, err := authcrypto.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	hash, err := authcrypto.HashAPIKey(rawKey)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}

	apiKey := &model.APIKey{
		UserID:    userID,
		Name:      name,
		KeyPrefix: authcrypto.APIKeyDisplayPrefix(rawKey),
		KeyHash:   hash,
	}
	if err := db.CreateAPIKey(apiKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	return rawKey
}

type middlewareAPIKeyStore interface {
	CreateAPIKey(apiKey *model.APIKey) error
}
