package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsMissingJWTSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("RBS_JWT_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "RBS_JWT_SECRET") {
		t.Fatalf("expected JWT secret validation error, got %v", err)
	}
}

func TestLoadRejectsMissingDataDir(t *testing.T) {
	setValidEnv(t)
	t.Setenv("RBS_DATA_DIR", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "RBS_DATA_DIR") {
		t.Fatalf("expected data dir validation error, got %v", err)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	setValidEnv(t)
	t.Setenv("RBS_PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load, got %v", err)
	}

	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/rbs-data" {
		t.Fatalf("expected data dir to match env, got %q", cfg.DataDir)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Fatalf("expected JWT secret to match env, got %q", cfg.JWTSecret)
	}
	if cfg.AdminUser != "admin" {
		t.Fatalf("expected admin user to match env, got %q", cfg.AdminUser)
	}
	if cfg.AdminPassword != "admin-pass" {
		t.Fatalf("expected admin password to match env, got %q", cfg.AdminPassword)
	}
}

func TestLoadReadsDotEnvWhenEnvironmentIsUnset(t *testing.T) {
	unsetEnv(t, "RBS_PORT")
	unsetEnv(t, "RBS_DATA_DIR")
	unsetEnv(t, "RBS_JWT_SECRET")
	unsetEnv(t, "RBS_ADMIN_USER")
	unsetEnv(t, "RBS_ADMIN_PASSWORD")

	tempDir := t.TempDir()
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWorkingDir)
	})

	dotEnv := strings.Join([]string{
		"RBS_PORT=9191",
		"RBS_DATA_DIR=/tmp/rbs-from-dotenv",
		"RBS_JWT_SECRET=dotenv-secret",
		"RBS_ADMIN_USER=dotenv-admin",
		"RBS_ADMIN_PASSWORD=dotenv-pass",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load from .env, got %v", err)
	}

	if cfg.Port != 9191 {
		t.Fatalf("expected port 9191 from .env, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/rbs-from-dotenv" {
		t.Fatalf("expected data dir from .env, got %q", cfg.DataDir)
	}
	if cfg.JWTSecret != "dotenv-secret" {
		t.Fatalf("expected JWT secret from .env, got %q", cfg.JWTSecret)
	}
	if cfg.AdminUser != "dotenv-admin" {
		t.Fatalf("expected admin user from .env, got %q", cfg.AdminUser)
	}
	if cfg.AdminPassword != "dotenv-pass" {
		t.Fatalf("expected admin password from .env, got %q", cfg.AdminPassword)
	}
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RBS_PORT", "8080")
	t.Setenv("RBS_DATA_DIR", "/tmp/rbs-data")
	t.Setenv("RBS_JWT_SECRET", "test-secret")
	t.Setenv("RBS_ADMIN_USER", "admin")
	t.Setenv("RBS_ADMIN_PASSWORD", "admin-pass")
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("cleanup unset %s: %v", key, err)
		}
	})
}