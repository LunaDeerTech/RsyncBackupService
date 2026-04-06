package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestBackupTargetCRUDAndHealthStatus(t *testing.T) {
	db := newTestDB(t)

	remote := &model.RemoteConfig{
		Name:           "ssh-remote",
		Type:           "ssh",
		Host:           "192.168.1.10",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: filepath.Join(t.TempDir(), "key.pem"),
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:           "primary-target",
		BackupType:     "rolling",
		StorageType:    "ssh",
		StoragePath:    "/srv/backup",
		RemoteConfigID: &remote.ID,
		HealthStatus:   "degraded",
		HealthMessage:  "health check pending",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}
	if target.ID == 0 {
		t.Fatal("CreateBackupTarget() ID = 0, want generated ID")
	}

	count, err := db.CountBackupTargets()
	if err != nil {
		t.Fatalf("CountBackupTargets() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountBackupTargets() = %d, want %d", count, 1)
	}

	loadedByName, err := db.GetBackupTargetByName(target.Name)
	if err != nil {
		t.Fatalf("GetBackupTargetByName() error = %v", err)
	}
	if loadedByName.ID != target.ID {
		t.Fatalf("GetBackupTargetByName().ID = %d, want %d", loadedByName.ID, target.ID)
	}

	loadedByID, err := db.GetBackupTargetByID(target.ID)
	if err != nil {
		t.Fatalf("GetBackupTargetByID() error = %v", err)
	}
	if loadedByID.StoragePath != target.StoragePath {
		t.Fatalf("GetBackupTargetByID().StoragePath = %q, want %q", loadedByID.StoragePath, target.StoragePath)
	}

	page, err := db.ListBackupTargetsPage(10, 0)
	if err != nil {
		t.Fatalf("ListBackupTargetsPage() error = %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("ListBackupTargetsPage() len = %d, want %d", len(page), 1)
	}

	target.BackupType = "cold"
	target.StorageType = "cloud"
	target.StoragePath = "oss://bucket/archive"
	target.RemoteConfigID = nil
	if err := db.UpdateBackupTarget(target); err != nil {
		t.Fatalf("UpdateBackupTarget() error = %v", err)
	}
	if target.StorageType != "cloud" {
		t.Fatalf("UpdateBackupTarget().StorageType = %q, want %q", target.StorageType, "cloud")
	}

	total := int64(1000)
	used := int64(250)
	if err := db.UpdateHealthStatus(target.ID, "healthy", "ok", &total, &used); err != nil {
		t.Fatalf("UpdateHealthStatus() error = %v", err)
	}
	updated, err := db.GetBackupTargetByID(target.ID)
	if err != nil {
		t.Fatalf("GetBackupTargetByID(updated) error = %v", err)
	}
	if updated.LastHealthCheck == nil {
		t.Fatal("LastHealthCheck = nil, want non-nil")
	}
	if updated.TotalCapacityBytes == nil || *updated.TotalCapacityBytes != total {
		t.Fatalf("TotalCapacityBytes = %v, want %d", updated.TotalCapacityBytes, total)
	}
	if updated.UsedCapacityBytes == nil || *updated.UsedCapacityBytes != used {
		t.Fatalf("UsedCapacityBytes = %v, want %d", updated.UsedCapacityBytes, used)
	}

	if err := db.DeleteBackupTarget(target.ID); err != nil {
		t.Fatalf("DeleteBackupTarget() error = %v", err)
	}
	if _, err := db.GetBackupTargetByID(target.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetBackupTargetByID() error = %v, want sql.ErrNoRows", err)
	}
}

func TestBackupTargetUsage(t *testing.T) {
	db := newTestDB(t)

	instanceResult, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"mysql-prod",
		"local",
		"/data/mysql",
		"online",
	)
	if err != nil {
		t.Fatalf("insert instance error = %v", err)
	}
	instanceID, err := instanceResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(instance) error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "nas-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "degraded",
		HealthMessage: "pending",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	inUse, err := db.IsBackupTargetInUse(target.ID)
	if err != nil {
		t.Fatalf("IsBackupTargetInUse() error = %v", err)
	}
	if inUse {
		t.Fatal("IsBackupTargetInUse() = true, want false")
	}

	if _, err := db.Exec(
		`INSERT INTO policies (instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retention_type, retention_value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		instanceID,
		"daily-backup",
		"rolling",
		target.ID,
		"cron",
		"0 0 * * *",
		true,
		false,
		false,
		nil,
		false,
		nil,
		"count",
		7,
	); err != nil {
		t.Fatalf("insert policy error = %v", err)
	}

	inUse, err = db.IsBackupTargetInUse(target.ID)
	if err != nil {
		t.Fatalf("IsBackupTargetInUse() second call error = %v", err)
	}
	if !inUse {
		t.Fatal("IsBackupTargetInUse() = false, want true")
	}

	usage, err := db.GetBackupTargetUsage(target.ID)
	if err != nil {
		t.Fatalf("GetBackupTargetUsage() error = %v", err)
	}
	if len(usage.Policies) != 1 || usage.Policies[0] != "daily-backup" {
		t.Fatalf("usage.Policies = %v, want [daily-backup]", usage.Policies)
	}
}
