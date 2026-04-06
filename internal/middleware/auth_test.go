package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestAuthStoresClaimsInContext(t *testing.T) {
	secret := "secret"
	token := signTestToken(t, authcrypto.Claims{
		UserID:    7,
		Email:     "viewer@example.com",
		Role:      "viewer",
		IssuedAt:  time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := MustGetUser(r.Context())
		if claims.UserID != 7 || claims.Email != "viewer@example.com" || claims.Role != "viewer" {
			t.Fatalf("claims = %+v, want populated viewer claims", claims)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	recorder := httptest.NewRecorder()

	handler := Auth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	handler.ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
}

func TestAuthRejectsExpiredToken(t *testing.T) {
	secret := "secret"
	token := signTestToken(t, authcrypto.Claims{
		UserID:    9,
		Email:     "viewer@example.com",
		Role:      "viewer",
		IssuedAt:  time.Now().Add(-2 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}, secret)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	handler.ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
}

func TestRequireAuthRejectsAnonymous(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	recorder := httptest.NewRecorder()

	RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
}

func TestRequireAdminRejectsViewer(t *testing.T) {
	secret := "secret"
	token := signTestToken(t, authcrypto.Claims{
		UserID:    11,
		Email:     "viewer@example.com",
		Role:      "viewer",
		IssuedAt:  time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/users", Auth(secret)(RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("admin-only handler should not be called")
	}))))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusForbidden, authErrorForbidden, "forbidden")
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	secret := "secret"
	token := signTestToken(t, authcrypto.Claims{
		UserID:    1,
		Email:     "admin@example.com",
		Role:      "admin",
		IssuedAt:  time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/users", Auth(secret)(RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestRequireInstanceAccessRejectsViewerWithoutPermission(t *testing.T) {
	db := newMiddlewareTestDB(t)
	viewer := createMiddlewareTestUser(t, db, "viewer@example.com", "viewer")
	instanceID := createMiddlewareTestInstance(t, db, "instance-one")
	secret := "secret"
	token := signAccessTokenForUser(t, viewer, secret)

	protected := Auth(secret)(RequireAuth(RequireInstanceAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("instance handler should not be called without permission")
	}))))

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instances/{id}", protected)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/"+itoa(instanceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	assertErrorResponse(t, recorder, http.StatusForbidden, authErrorForbidden, "forbidden")
}

func TestRequireInstanceAccessAllowsViewerWithPermission(t *testing.T) {
	db := newMiddlewareTestDB(t)
	viewer := createMiddlewareTestUser(t, db, "viewer@example.com", "viewer")
	instanceID := createMiddlewareTestInstance(t, db, "instance-one")
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	secret := "secret"
	token := signAccessTokenForUser(t, viewer, secret)

	protected := Auth(secret)(RequireAuth(RequireInstanceAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("id"); got != itoa(instanceID) {
			t.Fatalf("PathValue(id) = %q, want %q", got, itoa(instanceID))
		}
		w.WriteHeader(http.StatusNoContent)
	}))))

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instances/{id}", protected)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/"+itoa(instanceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestRequireInstanceAccessAllowsAdmin(t *testing.T) {
	secret := "secret"
	token := signTestToken(t, authcrypto.Claims{
		UserID:    1,
		Email:     "admin@example.com",
		Role:      "admin",
		IssuedAt:  time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)

	protected := Auth(secret)(RequireAuth(RequireInstanceAccess(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))))

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instances/{id}", protected)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/42", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestMustGetUserPanicsWithoutUser(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("MustGetUser() did not panic")
		}
	}()

	_ = MustGetUser(context.Background())
}

func newMiddlewareTestDB(t *testing.T) *store.DB {
	t.Helper()

	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})

	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}

	return db
}

func createMiddlewareTestUser(t *testing.T, db *store.DB, email string, role string) *model.User {
	t.Helper()

	user := &model.User{
		Email:        email,
		Name:         email,
		PasswordHash: "hash",
		Role:         role,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(%q) error = %v", email, err)
	}

	return user
}

func createMiddlewareTestInstance(t *testing.T, db *store.DB, name string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		name,
		"local",
		"/data/"+name,
		"idle",
	)
	if err != nil {
		t.Fatalf("insert instance %q error = %v", name, err)
	}

	instanceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error = %v", err)
	}

	return instanceID
}

func signAccessTokenForUser(t *testing.T, user *model.User, secret string) string {
	t.Helper()

	return signTestToken(t, authcrypto.Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		IssuedAt:  time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)
}

func signTestToken(t *testing.T, claims authcrypto.Claims, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	return signed
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, code int, message string) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d", recorder.Code, status)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	var payload response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Code != code {
		t.Fatalf("code = %d, want %d", payload.Code, code)
	}
	if payload.Message != message {
		t.Fatalf("message = %q, want %q", payload.Message, message)
	}
	if payload.Data != nil {
		t.Fatalf("data = %v, want nil", payload.Data)
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
