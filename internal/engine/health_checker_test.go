package engine

import (
	"os"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestHealthCheckerCheckTargetLocalHealthy(t *testing.T) {
	checker := NewHealthChecker(nil)
	path := t.TempDir()

	status, message, total, used, err := checker.CheckTarget(&model.BackupTarget{
		StorageType: "local",
		StoragePath: path,
	})
	if err != nil {
		t.Fatalf("CheckTarget() error = %v", err)
	}
	if status != "healthy" {
		t.Fatalf("status = %q, want %q", status, "healthy")
	}
	if message != "local target is healthy" {
		t.Fatalf("message = %q, want %q", message, "local target is healthy")
	}
	if total == nil || *total <= 0 {
		t.Fatalf("total = %v, want positive value", total)
	}
	if used == nil || *used < 0 {
		t.Fatalf("used = %v, want non-negative value", used)
	}
}

func TestHealthCheckerCheckTargetLocalMissingPath(t *testing.T) {
	checker := NewHealthChecker(nil)
	path := t.TempDir()
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(%q) error = %v", path, err)
	}

	status, message, total, used, err := checker.CheckTarget(&model.BackupTarget{
		StorageType: "local",
		StoragePath: path,
	})
	if err != nil {
		t.Fatalf("CheckTarget() error = %v", err)
	}
	if status != "unreachable" {
		t.Fatalf("status = %q, want %q", status, "unreachable")
	}
	if message != "local path does not exist" {
		t.Fatalf("message = %q, want %q", message, "local path does not exist")
	}
	if total != nil || used != nil {
		t.Fatalf("capacities = (%v, %v), want nil", total, used)
	}
}

func TestHealthCheckerCheckTargetLocalFilePath(t *testing.T) {
	checker := NewHealthChecker(nil)
	file, err := os.CreateTemp(t.TempDir(), "health-file")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	filePath := file.Name()
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	status, message, total, used, err := checker.CheckTarget(&model.BackupTarget{
		StorageType: "local",
		StoragePath: filePath,
	})
	if err != nil {
		t.Fatalf("CheckTarget() error = %v", err)
	}
	if status != "unreachable" {
		t.Fatalf("status = %q, want %q", status, "unreachable")
	}
	if message != "local path must be a directory" {
		t.Fatalf("message = %q, want %q", message, "local path must be a directory")
	}
	if total != nil || used != nil {
		t.Fatalf("capacities = (%v, %v), want nil", total, used)
	}
}
