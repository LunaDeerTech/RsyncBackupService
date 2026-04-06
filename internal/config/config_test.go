package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnvAndEnvOverrides(t *testing.T) {
	workingDir := t.TempDir()
	dotEnv := []byte("RBS_DATA_DIR='./state'\nRBS_PORT=8088\nRBS_JWT_SECRET='file-secret'\nRBS_WORKER_POOL_SIZE=5\nRBS_LOG_LEVEL=\"warn\"\nRBS_DEV_MODE=false\n")
	if err := os.WriteFile(filepath.Join(workingDir, ".env"), dotEnv, 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("RBS_PORT", "9090")
	t.Setenv("RBS_JWT_SECRET", "env-secret")
	t.Setenv("RBS_LOG_LEVEL", "debug")
	t.Setenv("RBS_DEV_MODE", "true")

	withWorkingDir(t, workingDir, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.DataDir != "./state" {
			t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "./state")
		}
		if cfg.Port != "9090" {
			t.Fatalf("Port = %q, want %q", cfg.Port, "9090")
		}
		if cfg.JWTSecret != "env-secret" {
			t.Fatalf("JWTSecret = %q, want %q", cfg.JWTSecret, "env-secret")
		}
		if cfg.WorkerPoolSize != 5 {
			t.Fatalf("WorkerPoolSize = %d, want %d", cfg.WorkerPoolSize, 5)
		}
		if cfg.LogLevel != "debug" {
			t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
		if !cfg.DevMode {
			t.Fatal("DevMode = false, want true")
		}
	})
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	workingDir := t.TempDir()
	dotEnv := []byte("RBS_DATA_DIR=./data\nRBS_PORT=8080\n")
	if err := os.WriteFile(filepath.Join(workingDir, ".env"), dotEnv, 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("RBS_JWT_SECRET", "")

	withWorkingDir(t, workingDir, func() {
		_, err := Load()
		if err == nil {
			t.Fatal("Load() error = nil, want missing JWT secret error")
		}
	})
}

func TestEnsureDataDirs(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "runtime-data")
	if err := EnsureDataDirs(dataDir); err != nil {
		t.Fatalf("EnsureDataDirs() error = %v", err)
	}

	paths := []string{
		dataDir,
		filepath.Join(dataDir, "keys"),
		filepath.Join(dataDir, "relay"),
		filepath.Join(dataDir, "temp"),
		filepath.Join(dataDir, "logs"),
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", path)
		}
	}
}

func TestLoadRejectsInvalidDevMode(t *testing.T) {
	workingDir := t.TempDir()
	dotEnv := []byte("RBS_JWT_SECRET=secret\nRBS_DEV_MODE=maybe\n")
	if err := os.WriteFile(filepath.Join(workingDir, ".env"), dotEnv, 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	withWorkingDir(t, workingDir, func() {
		_, err := Load()
		if err == nil {
			t.Fatal("Load() error = nil, want invalid boolean error")
		}
	})
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	fn()
}
