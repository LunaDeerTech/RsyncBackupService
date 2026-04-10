package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
)

func TestBackupTargetCRUDAndManualHealthCheck(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"))
	storagePath := t.TempDir()

	createResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets", map[string]any{
		"name":         "local-target",
		"backup_type":  "rolling",
		"storage_type": "local",
		"storage_path": storagePath,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/targets status = %d, want %d, body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	target, err := db.GetBackupTargetByName("local-target")
	if err != nil {
		t.Fatalf("GetBackupTargetByName() error = %v", err)
	}
	if target.HealthStatus != "degraded" || target.HealthMessage != "health check pending" {
		t.Fatalf("created target health = (%q, %q), want pending", target.HealthStatus, target.HealthMessage)
	}

	listResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/targets?page=1&page_size=10", nil, mustAccessTokenForUser(t, admin, "secret"))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/targets status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(listResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	var page struct {
		Items []model.BackupTarget `json:"items"`
		Total int64                `json:"total"`
	}
	if err := json.Unmarshal(envelope.Data, &page); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("list payload = %+v, want one target", page)
	}

	healthResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets/"+itoa(target.ID)+"/health-check", nil, mustAccessTokenForUser(t, admin, "secret"))
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/targets/{id}/health-check status = %d, want %d, body = %s", healthResponse.Code, http.StatusOK, healthResponse.Body.String())
	}

	checked, err := db.GetBackupTargetByID(target.ID)
	if err != nil {
		t.Fatalf("GetBackupTargetByID() after health check error = %v", err)
	}
	if checked.HealthStatus != "healthy" {
		t.Fatalf("checked.HealthStatus = %q, want %q", checked.HealthStatus, "healthy")
	}
	if checked.LastHealthCheck == nil {
		t.Fatal("LastHealthCheck = nil, want non-nil")
	}
	if checked.TotalCapacityBytes == nil || checked.UsedCapacityBytes == nil {
		t.Fatalf("capacities = (%v, %v), want non-nil", checked.TotalCapacityBytes, checked.UsedCapacityBytes)
	}

	updateResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/targets/"+itoa(target.ID), map[string]any{
		"name":         "local-target-renamed",
		"backup_type":  "cold",
		"storage_type": "local",
		"storage_path": storagePath,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/targets/{id} status = %d, want %d, body = %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}

	updated, err := db.GetBackupTargetByID(target.ID)
	if err != nil {
		t.Fatalf("GetBackupTargetByID(updated) error = %v", err)
	}
	if updated.Name != "local-target-renamed" || updated.BackupType != "cold" {
		t.Fatalf("updated target = %+v, want renamed cold target", updated)
	}

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/targets/"+itoa(target.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/targets/{id} status = %d, want %d, body = %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	if _, err := db.GetBackupTargetByID(target.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetBackupTargetByID(deleted) error = %v, want sql.ErrNoRows", err)
	}
}

func TestBackupTargetValidationAndDeleteInUse(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"))

	invalidCombo := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets", map[string]any{
		"name":         "invalid-cloud",
		"backup_type":  "rolling",
		"storage_type": "openlist",
		"storage_path": "oss://bucket/path",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, invalidCombo, http.StatusBadRequest, authErrorInvalidRequest, "rolling backups only support local or ssh storage")

	missingRemote := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets", map[string]any{
		"name":         "ssh-without-remote",
		"backup_type":  "rolling",
		"storage_type": "ssh",
		"storage_path": "/srv/backup",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, missingRemote, http.StatusBadRequest, authErrorInvalidRequest, "remote_config_id is required for ssh storage")

	remote := &model.RemoteConfig{
		Name:           "ssh-remote",
		Type:           "ssh",
		Host:           "192.168.1.20",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: filepath.Join(t.TempDir(), "id_rsa"),
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	validSSH := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets", map[string]any{
		"name":             "ssh-target",
		"backup_type":      "rolling",
		"storage_type":     "ssh",
		"storage_path":     "/srv/backup",
		"remote_config_id": remote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if validSSH.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/targets ssh status = %d, want %d, body = %s", validSSH.Code, http.StatusCreated, validSSH.Body.String())
	}

	openListConfig, err := openlist.EncodeStoredConfig("secret", "")
	if err != nil {
		t.Fatalf("EncodeStoredConfig() error = %v", err)
	}
	openListRemote := &model.RemoteConfig{
		Name:          "openlist-remote",
		Type:          "openlist",
		Host:          "https://openlist.example.com",
		Username:      "admin",
		CloudProvider: stringPtr("openlist"),
		CloudConfig:   openListConfig,
	}
	if err := db.CreateRemoteConfig(openListRemote); err != nil {
		t.Fatalf("CreateRemoteConfig(openlist) error = %v", err)
	}

	validOpenList := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/targets", map[string]any{
		"name":             "openlist-target",
		"backup_type":      "cold",
		"storage_type":     "openlist",
		"storage_path":     "/archive/backups",
		"remote_config_id": openListRemote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if validOpenList.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/targets openlist status = %d, want %d, body = %s", validOpenList.Code, http.StatusCreated, validOpenList.Body.String())
	}

	localTarget := &model.BackupTarget{
		Name:          "protected-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "degraded",
		HealthMessage: "pending",
	}
	if err := db.CreateBackupTarget(localTarget); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	if _, err := db.Exec(
		`INSERT INTO policies (instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retention_type, retention_value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		instanceID,
		"daily-backup",
		"rolling",
		localTarget.ID,
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

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/targets/"+itoa(localTarget.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusBadRequest {
		t.Fatalf("DELETE /api/v1/targets/{id} status = %d, want %d, body = %s", deleteResponse.Code, http.StatusBadRequest, deleteResponse.Body.String())
	}
	if !strings.Contains(deleteResponse.Body.String(), "daily-backup") {
		t.Fatalf("delete response = %s, want policy usage details", deleteResponse.Body.String())
	}
}
