package service

import (
	"context"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestLoginIssuesAccessAndRefreshTokens(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	tokens, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Fatal("expected access token")
	}
	if tokens.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	identity, err := fixture.auth.AuthenticateAccessToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}
	if identity.UserID != fixture.admin.ID {
		t.Fatalf("expected user id %d, got %d", fixture.admin.ID, identity.UserID)
	}
	if !identity.IsAdmin {
		t.Fatal("expected seeded admin identity")
	}

	refreshed, err := fixture.auth.Refresh(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatal("expected refreshed access and refresh tokens")
	}
	if refreshed.AccessToken == tokens.AccessToken {
		t.Fatal("expected refreshed access token to differ from original")
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	tokens, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := fixture.auth.Refresh(context.Background(), tokens.AccessToken); err == nil {
		t.Fatal("expected refresh with access token to fail")
	}
}

func TestRefreshRejectsReplayedRefreshToken(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	tokens, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := fixture.auth.Refresh(context.Background(), tokens.RefreshToken); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if _, err := fixture.auth.Refresh(context.Background(), tokens.RefreshToken); err == nil {
		t.Fatal("expected replayed refresh token to be rejected")
	}
}

func TestVerifyPasswordIssuesConsumableToken(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	verifyToken, err := fixture.auth.VerifyPassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if verifyToken == "" {
		t.Fatal("expected verify token")
	}

	if err := fixture.auth.ConsumeVerifyToken(context.Background(), fixture.admin.ID, verifyToken); err != nil {
		t.Fatalf("consume verify token: %v", err)
	}

	if err := fixture.auth.ConsumeVerifyToken(context.Background(), fixture.admin.ID, verifyToken); err == nil {
		t.Fatal("expected verify token to be one-time use")
	}
	if _, err := fixture.auth.VerifyPassword(context.Background(), fixture.admin.ID, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail verification")
	}

	expiredToken, err := fixture.auth.VerifyPassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("verify password for expired token: %v", err)
	}

	fixture.auth.clock = func() time.Time {
		return fixture.now.Add(6 * time.Minute)
	}
	if err := fixture.auth.ConsumeVerifyToken(context.Background(), fixture.admin.ID, expiredToken); err == nil {
		t.Fatal("expected expired verify token to fail")
	}
}

func TestResetPasswordRevokesExistingRefreshTokens(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)
	managedUser := createAuthServiceTestUser(t, fixture.db, "operator", "old-secret", false)

	tokens, err := fixture.auth.Login(context.Background(), managedUser.Username, "old-secret")
	if err != nil {
		t.Fatalf("login before reset: %v", err)
	}

	if err := fixture.auth.ResetPassword(context.Background(), managedUser.ID, "new-secret"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if _, err := fixture.auth.Refresh(context.Background(), tokens.RefreshToken); err == nil {
		t.Fatal("expected old refresh token to be rejected after password reset")
	}
}

func TestChangePasswordRevokesOutstandingVerifyTokens(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	verifyToken, err := fixture.auth.VerifyPassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("issue verify token: %v", err)
	}

	if err := fixture.auth.ChangePassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword, "new-secret"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	if err := fixture.auth.ConsumeVerifyToken(context.Background(), fixture.admin.ID, verifyToken); err == nil {
		t.Fatal("expected outstanding verify token to be revoked after password change")
	}
	if _, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, fixture.cfg.AdminPassword); err == nil {
		t.Fatal("expected old password login to fail after password change")
	}
	if _, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, "new-secret"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestChangePasswordRevokesExistingAccessTokens(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)

	tokens, err := fixture.auth.Login(context.Background(), fixture.cfg.AdminUser, fixture.cfg.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := fixture.auth.ChangePassword(context.Background(), fixture.admin.ID, fixture.cfg.AdminPassword, "new-secret"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	if _, err := fixture.auth.AuthenticateAccessToken(context.Background(), tokens.AccessToken); err == nil {
		t.Fatal("expected old access token to be rejected after password change")
	}
}

func TestManagedUserPasswordsPreserveSignificantWhitespace(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)
	userService := NewUserService(fixture.db, fixture.auth)

	managedUser, err := userService.Create(context.Background(), "space-user", " secret ", false)
	if err != nil {
		t.Fatalf("create managed user: %v", err)
	}

	if _, err := fixture.auth.Login(context.Background(), managedUser.Username, "secret"); err == nil {
		t.Fatal("expected trimmed password login to fail")
	}
	if _, err := fixture.auth.Login(context.Background(), managedUser.Username, " secret "); err != nil {
		t.Fatalf("login with exact whitespace-preserved password: %v", err)
	}
}

type authServiceTestFixture struct {
	db    *gorm.DB
	auth  *AuthService
	admin model.User
	cfg   config.Config
	now   time.Time
}

func newAuthServiceTestFixture(t *testing.T) authServiceTestFixture {
	t.Helper()

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

	auth := NewAuthService(db, "test-jwt-secret")
	now := time.Now().UTC()
	auth.clock = func() time.Time {
		return now
	}

	return authServiceTestFixture{
		db:    db,
		auth:  auth,
		admin: admin,
		cfg:   cfg,
		now:   now,
	}
}

func createAuthServiceTestUser(t *testing.T, db *gorm.DB, username, password string, isAdmin bool) model.User {
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