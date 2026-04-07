package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestRestoreBackupRejectsWrongPassword(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, false)
	response := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/restore", map[string]any{
		"restore_type":  "source",
		"instance_name": "mysql-prod",
		"password":      "WrongPass123",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, response, http.StatusBadRequest, authErrorInvalidRequest, "password is incorrect")
}

func TestRestoreBackupCreatesQueuedRestoreTask(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, true)
	response := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/restore", map[string]any{
		"restore_type":   "custom",
		"target_path":    "/restore/mysql-prod",
		"instance_name":  "mysql-prod",
		"password":       "AdminPass123",
		"encryption_key": "Cold#123",
	}, mustAccessTokenForUser(t, admin, "secret"))
	if response.Code != http.StatusCreated {
		t.Fatalf("POST restore status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Task model.Task `json:"task"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Task.Type != "restore" || payload.Task.RestoreType != "custom" || payload.Task.TargetPath != "/restore/mysql-prod" {
		t.Fatalf("restore task payload = %+v, want queued custom restore task", payload.Task)
	}
	loadedTask, err := db.GetTaskByID(payload.Task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "queued" {
		t.Fatalf("loaded task status = %q, want queued", loadedTask.Status)
	}
}

func TestGenerateBackupDownloadURLAndConsumeTokenOnce(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, false)
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readonly"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/download", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET download URL status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.URL == "" {
		t.Fatal("download URL = empty, want temporary URL")
	}

	request := httptest.NewRequest(http.MethodGet, payload.URL, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET token download status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "cold-backup-bytes" {
		t.Fatalf("download body = %q, want %q", recorder.Body.String(), "cold-backup-bytes")
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, payload.URL, nil))
	assertAPIError(t, second, http.StatusForbidden, 40302, "download token is invalid")
}

func createColdBackupForHandlerTests(t *testing.T, db *store.DB, encrypted bool) (int64, int64) {
	t.Helper()

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	target := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "nightly-cold",
		Type:           "cold",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		Compression:    true,
		Encryption:     encrypted,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if encrypted {
		hash := authcrypto.HashEncryptionKey("Cold#123")
		policy.EncryptionKeyHash = &hash
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	artifactPath := filepath.Join(t.TempDir(), "mysql-prod-20260407-210000.tar.gz")
	if encrypted {
		artifactPath += ".enc"
	}
	if err := os.WriteFile(artifactPath, []byte("cold-backup-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifact) error = %v", err)
	}

	completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policy.ID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            "cold",
		Status:          "success",
		SnapshotPath:    artifactPath,
		BackupSizeBytes: int64(len("cold-backup-bytes")),
		ActualSizeBytes: int64(len("cold-backup-bytes")),
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: 60,
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	return instanceID, backup.ID
}