package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestRequireJWTRejectsMissingToken(t *testing.T) {
	fixture := newMiddlewareTestFixture(t)

	router := gin.New()
	router.Use(InjectServices(fixture.authService, fixture.permissionService))
	router.GET("/protected", RequireJWT(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestRequireVerifyTokenRejectsMissingToken(t *testing.T) {
	fixture := newMiddlewareTestFixture(t)
	accessToken := loginMiddlewareTestUser(t, fixture.authService, fixture.cfg.AdminUser, fixture.cfg.AdminPassword)

	router := gin.New()
	router.Use(InjectServices(fixture.authService, fixture.permissionService))
	router.POST("/instances/:id/restore", RequireJWT(), RequireInstanceRole(service.RoleAdmin), RequireVerifyToken(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/instances/1/restore", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestRequireInstanceRoleAllowsAdminOrAssignedRole(t *testing.T) {
	fixture := newMiddlewareTestFixture(t)
	viewerUser := createMiddlewareTestUser(t, fixture.db, "viewer", "viewer-secret", false)
	permission := model.InstancePermission{UserID: viewerUser.ID, InstanceID: fixture.instance.ID, Role: service.RoleViewer}
	if err := fixture.db.Create(&permission).Error; err != nil {
		t.Fatalf("create instance permission: %v", err)
	}

	adminAccessToken := loginMiddlewareTestUser(t, fixture.authService, fixture.cfg.AdminUser, fixture.cfg.AdminPassword)
	adminVerifyToken, err := fixture.authService.VerifyPassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("verify admin password: %v", err)
	}

	viewerAccessToken := loginMiddlewareTestUser(t, fixture.authService, viewerUser.Username, "viewer-secret")
	viewerVerifyToken, err := fixture.authService.VerifyPassword(context.Background(), viewerUser.ID, "viewer-secret")
	if err != nil {
		t.Fatalf("verify viewer password: %v", err)
	}

	router := gin.New()
	router.Use(InjectServices(fixture.authService, fixture.permissionService))
	router.GET("/instances/:id/view", RequireJWT(), RequireInstanceRole(service.RoleViewer), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/instances/:id/restore", RequireJWT(), RequireInstanceRole(service.RoleAdmin), RequireVerifyToken(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	viewerReq := httptest.NewRequest(http.MethodGet, "/instances/1/view", nil)
	viewerReq.Header.Set("Authorization", "Bearer "+viewerAccessToken)
	viewerResp := httptest.NewRecorder()
	router.ServeHTTP(viewerResp, viewerReq)
	if viewerResp.Code != http.StatusNoContent {
		t.Fatalf("expected viewer route to allow assigned viewer, got %d", viewerResp.Code)
	}

	adminReq := httptest.NewRequest(http.MethodPost, "/instances/1/restore", nil)
	adminReq.Header.Set("Authorization", "Bearer "+adminAccessToken)
	adminReq.Header.Set("X-Verify-Token", adminVerifyToken)
	adminResp := httptest.NewRecorder()
	router.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusNoContent {
		t.Fatalf("expected admin route to allow super admin, got %d", adminResp.Code)
	}

	viewerAdminReq := httptest.NewRequest(http.MethodPost, "/instances/1/restore", nil)
	viewerAdminReq.Header.Set("Authorization", "Bearer "+viewerAccessToken)
	viewerAdminReq.Header.Set("X-Verify-Token", viewerVerifyToken)
	viewerAdminResp := httptest.NewRecorder()
	router.ServeHTTP(viewerAdminResp, viewerAdminReq)
	if viewerAdminResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer restore route to be forbidden, got %d", viewerAdminResp.Code)
	}
}

type middlewareTestFixture struct {
	db                *gorm.DB
	authService       *service.AuthService
	permissionService *service.PermissionService
	cfg               config.Config
	admin             model.User
	instance          model.BackupInstance
}

func newMiddlewareTestFixture(t *testing.T) middlewareTestFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}
	if err := repository.MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	var admin model.User
	if err := db.Where("username = ?", cfg.AdminUser).First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create backup instance: %v", err)
	}

	return middlewareTestFixture{
		db:                db,
		authService:       service.NewAuthService(db, "test-jwt-secret"),
		permissionService: service.NewPermissionService(db),
		cfg:               cfg,
		admin:             admin,
		instance:          instance,
	}
}

func loginMiddlewareTestUser(t *testing.T, authService *service.AuthService, username, password string) string {
	t.Helper()

	tokens, err := authService.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("login user %s: %v", username, err)
	}

	return tokens.AccessToken
}

func createMiddlewareTestUser(t *testing.T, db *gorm.DB, username, password string, isAdmin bool) model.User {
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