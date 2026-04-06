package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestRemoteConfigCRUD(t *testing.T) {
	db := newTestDB(t)

	provider := "aliyun"
	cloudConfig := `{"region":"cn-hangzhou"}`
	remote := &model.RemoteConfig{
		Name:           "prod-ssh",
		Type:           "ssh",
		Host:           "192.168.10.2",
		Port:           22,
		Username:       "root",
		PrivateKeyPath: filepath.Join(t.TempDir(), "keys", "first.pem"),
		CloudProvider:  &provider,
		CloudConfig:    &cloudConfig,
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}
	if remote.ID == 0 {
		t.Fatal("CreateRemoteConfig() ID = 0, want generated ID")
	}

	count, err := db.CountRemoteConfigs()
	if err != nil {
		t.Fatalf("CountRemoteConfigs() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountRemoteConfigs() = %d, want %d", count, 1)
	}

	loadedByName, err := db.GetRemoteConfigByName(remote.Name)
	if err != nil {
		t.Fatalf("GetRemoteConfigByName() error = %v", err)
	}
	if loadedByName.ID != remote.ID {
		t.Fatalf("GetRemoteConfigByName().ID = %d, want %d", loadedByName.ID, remote.ID)
	}

	loadedByID, err := db.GetRemoteConfigByID(remote.ID)
	if err != nil {
		t.Fatalf("GetRemoteConfigByID() error = %v", err)
	}
	if loadedByID.Host != remote.Host {
		t.Fatalf("GetRemoteConfigByID().Host = %q, want %q", loadedByID.Host, remote.Host)
	}

	page, err := db.ListRemoteConfigsPage(10, 0)
	if err != nil {
		t.Fatalf("ListRemoteConfigsPage() error = %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("ListRemoteConfigsPage() len = %d, want %d", len(page), 1)
	}

	remote.Name = "prod-cloud"
	remote.Type = "cloud"
	remote.Host = ""
	remote.Port = 0
	remote.Username = ""
	remote.PrivateKeyPath = ""
	if err := db.UpdateRemoteConfig(remote); err != nil {
		t.Fatalf("UpdateRemoteConfig() error = %v", err)
	}
	if remote.Type != "cloud" {
		t.Fatalf("UpdateRemoteConfig().Type = %q, want %q", remote.Type, "cloud")
	}

	if err := db.DeleteRemoteConfig(remote.ID); err != nil {
		t.Fatalf("DeleteRemoteConfig() error = %v", err)
	}
	if _, err := db.GetRemoteConfigByID(remote.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetRemoteConfigByID() error = %v, want sql.ErrNoRows", err)
	}
}

func TestRemoteConfigUsage(t *testing.T) {
	db := newTestDB(t)

	remote := &model.RemoteConfig{
		Name:           "shared-remote",
		Type:           "ssh",
		Host:           "10.0.0.8",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: filepath.Join(t.TempDir(), "keys", "shared.pem"),
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	inUse, err := db.IsRemoteConfigInUse(remote.ID)
	if err != nil {
		t.Fatalf("IsRemoteConfigInUse() error = %v", err)
	}
	if inUse {
		t.Fatal("IsRemoteConfigInUse() = true, want false")
	}

	if _, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, remote_config_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"mysql-prod",
		"ssh",
		"/data/mysql",
		remote.ID,
		"online",
	); err != nil {
		t.Fatalf("insert instance error = %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO backup_targets (name, backup_type, storage_type, storage_path, remote_config_id, total_capacity_bytes, used_capacity_bytes, health_status, health_message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"nas-target",
		"rolling",
		"ssh",
		"/backup",
		remote.ID,
		100,
		20,
		"healthy",
		"ok",
	); err != nil {
		t.Fatalf("insert backup target error = %v", err)
	}

	inUse, err = db.IsRemoteConfigInUse(remote.ID)
	if err != nil {
		t.Fatalf("IsRemoteConfigInUse() second call error = %v", err)
	}
	if !inUse {
		t.Fatal("IsRemoteConfigInUse() = false, want true")
	}

	usage, err := db.GetRemoteConfigUsage(remote.ID)
	if err != nil {
		t.Fatalf("GetRemoteConfigUsage() error = %v", err)
	}
	if len(usage.Instances) != 1 || usage.Instances[0] != "mysql-prod" {
		t.Fatalf("usage.Instances = %v, want [mysql-prod]", usage.Instances)
	}
	if len(usage.BackupTargets) != 1 || usage.BackupTargets[0] != "nas-target" {
		t.Fatalf("usage.BackupTargets = %v, want [nas-target]", usage.BackupTargets)
	}

}
