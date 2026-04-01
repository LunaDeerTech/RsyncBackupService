package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestLoginIssuesAccessAndRefreshTokens(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AccessToken == "" || body.RefreshToken == "" {
		t.Fatal("expected both access and refresh tokens")
	}
}

func TestVerifyReturnsOneTimeToken(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(`{"password":"secret"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		VerifyToken string `json:"verify_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.VerifyToken == "" {
		t.Fatal("expected verify token")
	}
}

func TestAdminUserAPIRejectsNonAdmin(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	regularUser := createAPITestUser(t, fixture.db, "operator", "password", false)
	accessToken := loginForAccessToken(t, router, regularUser.Username, "password")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
}

func TestFailedVerifyAuditUsesExplicitAction(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(`{"password":"wrong-password"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}

	var auditLog model.AuditLog
	if err := fixture.db.Order("id DESC").First(&auditLog).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	if auditLog.Action != "auth.verify" {
		t.Fatalf("expected audit action auth.verify, got %q", auditLog.Action)
	}
}

func TestFailedAdminPasswordResetAuditUsesExplicitAction(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	regularUser := createAPITestUser(t, fixture.db, "operator", "password", false)
	accessToken := loginForAccessToken(t, router, regularUser.Username, "password")

	req := httptest.NewRequest(http.MethodPut, "/api/users/1/password", bytes.NewBufferString(`{"password":"reset-secret"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}

	var auditLog model.AuditLog
	if err := fixture.db.Order("id DESC").First(&auditLog).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	if auditLog.Action != "users.password.reset" {
		t.Fatalf("expected audit action users.password.reset, got %q", auditLog.Action)
	}
}

func TestAdminPasswordResetRequiresVerifyToken(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	req := httptest.NewRequest(http.MethodPut, "/api/users/1/password", bytes.NewBufferString(`{"password":"reset-secret"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

type authAPITestFixture struct {
	db *gorm.DB
}

func newAuthTestRouter(t *testing.T) (http.Handler, authAPITestFixture) {
	t.Helper()

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}
	if err := repository.MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	authService := service.NewAuthService(db, "test-jwt-secret")
	userService := service.NewUserService(db, authService)
	permissionService := service.NewPermissionService(db)
	auditRepo := repository.NewAuditLogRepository(db)

	router := NewRouter(Dependencies{
		AuthService:       authService,
		UserService:       userService,
		PermissionService: permissionService,
		AuditLogRepo:      auditRepo,
	})

	return router, authAPITestFixture{db: db}
}

func loginForAccessToken(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	return body.AccessToken
}

func createAPITestUser(t *testing.T, db *gorm.DB, username, password string, isAdmin bool) model.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := model.User{
		Username:     username,
		PasswordHash: string(passwordHash),
		IsAdmin:      isAdmin,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}