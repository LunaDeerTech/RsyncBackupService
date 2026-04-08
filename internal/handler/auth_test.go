package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestRegisterAssignsAdminThenViewer(t *testing.T) {
	db := newAuthTestDB(t)
	sender := newRecordingPasswordSender()
	router := NewRouter(
		db,
		WithJWTSecret("secret"),
		withPasswordSender(sender),
		withPasswordGenerator(sequencePasswordGenerator("AdminPass123", "ViewerPass123")),
	)

	first := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "admin@example.com"})
	if first.Code != http.StatusOK {
		t.Fatalf("first register status = %d, want %d", first.Code, http.StatusOK)
	}
	firstUser, err := db.GetUserByEmail("admin@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail(first) error = %v", err)
	}
	if firstUser.Role != "admin" {
		t.Fatalf("first user role = %q, want %q", firstUser.Role, "admin")
	}
	if sender.PasswordFor("admin@example.com") != "AdminPass123" {
		t.Fatalf("recorded password = %q, want %q", sender.PasswordFor("admin@example.com"), "AdminPass123")
	}

	second := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "viewer@example.com"})
	if second.Code != http.StatusOK {
		t.Fatalf("second register status = %d, want %d", second.Code, http.StatusOK)
	}
	secondUser, err := db.GetUserByEmail("viewer@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail(second) error = %v", err)
	}
	if secondUser.Role != "viewer" {
		t.Fatalf("second user role = %q, want %q", secondUser.Role, "viewer")
	}
}

func TestLoginAndRefresh(t *testing.T) {
	db := newAuthTestDB(t)
	sender := newRecordingPasswordSender()
	router := NewRouter(
		db,
		WithJWTSecret("secret"),
		withPasswordSender(sender),
		withPasswordGenerator(sequencePasswordGenerator("LoginPass123")),
	)

	register := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "user@example.com"})
	if register.Code != http.StatusOK {
		t.Fatalf("register status = %d, want %d", register.Code, http.StatusOK)
	}

	login := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": sender.PasswordFor("user@example.com"),
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", login.Code, http.StatusOK)
	}

	var loginEnvelope apiEnvelope
	if err := json.Unmarshal(login.Body.Bytes(), &loginEnvelope); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	var loginData struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		User         json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(loginEnvelope.Data, &loginData); err != nil {
		t.Fatalf("decode login data: %v", err)
	}

	claims, err := authcrypto.ParseToken(loginData.AccessToken, "secret")
	if err != nil {
		t.Fatalf("ParseToken(access) error = %v", err)
	}
	if claims.UserID == 0 || claims.Email != "user@example.com" || claims.Role != "admin" {
		t.Fatalf("access token claims = %+v, want populated user@example.com/admin claims", claims)
	}

	refresh := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": loginData.RefreshToken,
	})
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want %d", refresh.Code, http.StatusOK)
	}

	var refreshEnvelope apiEnvelope
	if err := json.Unmarshal(refresh.Body.Bytes(), &refreshEnvelope); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	var refreshData struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(refreshEnvelope.Data, &refreshData); err != nil {
		t.Fatalf("decode refresh data: %v", err)
	}
	if refreshData.AccessToken == "" {
		t.Fatal("refresh access token = empty, want token")
	}
}

func TestLoginRateLimitAfterFiveFailures(t *testing.T) {
	db := newAuthTestDB(t)
	router := NewRouter(
		db,
		WithJWTSecret("secret"),
	)

	register := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "locked@example.com"})
	if register.Code != http.StatusOK {
		t.Fatalf("register status = %d, want %d", register.Code, http.StatusOK)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		response := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email":    "locked@example.com",
			"password": "wrong-password",
		})
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
		}
	}

	locked := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "locked@example.com",
		"password": "wrong-password",
	})
	if locked.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d, want %d", locked.Code, http.StatusTooManyRequests)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(locked.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode locked response: %v", err)
	}
	if envelope.Message != "too many login attempts, try again later" {
		t.Fatalf("locked message = %q, want %q", envelope.Message, "too many login attempts, try again later")
	}
}

func TestRegisterRespectsRegistrationSwitchAfterBootstrap(t *testing.T) {
	db := newAuthTestDB(t)
	sender := newRecordingPasswordSender()
	systemConfigs := service.NewSystemConfigService(db, authcrypto.DeriveAESKey("secret"))
	if err := systemConfigs.UpdateRegistrationEnabled(false); err != nil {
		t.Fatalf("UpdateRegistrationEnabled(false) error = %v", err)
	}
	router := NewRouter(
		db,
		WithJWTSecret("secret"),
		WithSystemConfigService(systemConfigs),
		withPasswordSender(sender),
		withPasswordGenerator(sequencePasswordGenerator("AdminPass123", "ViewerPass123")),
	)

	status := performAuthRequest(t, router, http.MethodGet, "/api/v1/system/registration", nil)
	if status.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/registration status = %d, want %d", status.Code, http.StatusOK)
	}

	first := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "admin@example.com"})
	if first.Code != http.StatusOK {
		t.Fatalf("first register status = %d, want %d", first.Code, http.StatusOK)
	}

	second := performAuthRequest(t, router, http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "viewer@example.com"})
	assertAPIError(t, second, http.StatusForbidden, authErrorRegistrationOff, "registration is disabled")
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type recordingPasswordSender struct {
	mu        sync.Mutex
	passwords map[string]string
}

func newRecordingPasswordSender() *recordingPasswordSender {
	return &recordingPasswordSender{passwords: make(map[string]string)}
}

func (s *recordingPasswordSender) SendPassword(_ context.Context, email, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.passwords[email] = password
	return nil
}

func (s *recordingPasswordSender) PasswordFor(email string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.passwords[email]
}

func newAuthTestDB(t *testing.T) *store.DB {
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

func performAuthRequest(t *testing.T, handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	return recorder
}

func sequencePasswordGenerator(passwords ...string) func() (string, error) {
	var (
		mu    sync.Mutex
		index int
	)

	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if index >= len(passwords) {
			return "", nil
		}

		password := passwords[index]
		index++
		return password, nil
	}
}
