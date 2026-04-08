package service

import (
	"net/smtp"
	"strings"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/store"
)

func TestSystemConfigServiceSMTPMaskingAndDecryption(t *testing.T) {
	db := newSMTPServiceTestDB(t)
	service := NewSystemConfigService(db, authcrypto.DeriveAESKey("secret"))

	if err := service.UpdateSMTPConfig(&SMTPConfig{Host: "smtp.example.com", Port: 587, Username: "user", Password: "smtp-pass", From: "noreply@example.com"}); err != nil {
		t.Fatalf("UpdateSMTPConfig() error = %v", err)
	}

	stored, err := db.GetSystemConfig(smtpPasswordKey)
	if err != nil {
		t.Fatalf("GetSystemConfig(smtp.password) error = %v", err)
	}
	if stored == "smtp-pass" {
		t.Fatal("smtp password stored in plaintext")
	}

	masked, err := service.GetSMTPConfig()
	if err != nil {
		t.Fatalf("GetSMTPConfig() error = %v", err)
	}
	if masked.Password != maskedSMTPPassword {
		t.Fatalf("masked password = %q, want %q", masked.Password, maskedSMTPPassword)
	}

	raw, err := service.GetSMTPConfigWithPassword()
	if err != nil {
		t.Fatalf("GetSMTPConfigWithPassword() error = %v", err)
	}
	if raw.Password != "smtp-pass" {
		t.Fatalf("decrypted password = %q, want %q", raw.Password, "smtp-pass")
	}
}

func TestSystemConfigServiceRegistrationDefaultsTrue(t *testing.T) {
	db := newSMTPServiceTestDB(t)
	service := NewSystemConfigService(db, authcrypto.DeriveAESKey("secret"))

	enabled, err := service.GetRegistrationEnabled()
	if err != nil {
		t.Fatalf("GetRegistrationEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatal("default registration enabled = false, want true")
	}

	if err := service.UpdateRegistrationEnabled(false); err != nil {
		t.Fatalf("UpdateRegistrationEnabled() error = %v", err)
	}
	enabled, err = service.GetRegistrationEnabled()
	if err != nil {
		t.Fatalf("GetRegistrationEnabled(updated) error = %v", err)
	}
	if enabled {
		t.Fatal("registration enabled = true, want false")
	}
}

func TestSystemConfigServiceTestSMTP(t *testing.T) {
	db := newSMTPServiceTestDB(t)
	service := NewSystemConfigService(db, authcrypto.DeriveAESKey("secret"))
	called := false
	service.SetMailer(func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		called = true
		if addr != "smtp.example.com:587" || from != "noreply@example.com" || len(to) != 1 || to[0] != "user@example.com" {
			t.Fatalf("mailer args = %q %q %+v", addr, from, to)
		}
		if !strings.Contains(string(msg), "SMTP 测试邮件") {
			t.Fatalf("message = %q, want test subject", msg)
		}
		return nil
	})

	err := service.TestSMTP(&SMTPConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com"}, "user@example.com")
	if err != nil {
		t.Fatalf("TestSMTP() error = %v", err)
	}
	if !called {
		t.Fatal("TestSMTP() did not call mailer")
	}
}

func newSMTPServiceTestDB(t *testing.T) *store.DB {
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