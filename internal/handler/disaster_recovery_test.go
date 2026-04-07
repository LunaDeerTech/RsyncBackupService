package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	servicepkg "rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestGetDisasterRecoveryScore(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin-dr@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"))

	instance := &model.Instance{Name: "mysql-dr", SourceType: "local", SourcePath: "/srv/mysql", Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	target := &model.BackupTarget{
		Name:          "dr-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}
	policyID := createHandlerTestPolicy(t, db, instance.ID, target.ID, "dr-policy")
	insertHandlerTestBackupAt(t, db, instance.ID, policyID, "success", 100, 80, time.Date(2026, 4, 7, 11, 45, 0, 0, time.UTC))

	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instance.ID)+"/disaster-recovery", nil, mustAccessTokenForUser(t, admin, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /disaster-recovery status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload servicepkg.DisasterRecoveryScore
	var apiResponse Response
	if err := json.Unmarshal(response.Body.Bytes(), &apiResponse); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	encoded, err := json.Marshal(apiResponse.Data)
	if err != nil {
		t.Fatalf("json.Marshal(data) error = %v", err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("json.Unmarshal(data) error = %v", err)
	}
	if payload.Level == "" {
		t.Fatal("Level = empty, want value")
	}
	if payload.CalculatedAt.IsZero() {
		t.Fatal("CalculatedAt = zero, want timestamp")
	}
	if payload.Total <= 0 {
		t.Fatalf("Total = %v, want > 0", payload.Total)
	}
	if payload.Freshness <= 0 || payload.RecoveryPoints <= 0 || payload.Redundancy <= 0 {
		t.Fatalf("subscores = %+v, want positive values", payload)
	}
}

func insertHandlerTestBackupAt(t *testing.T, db *store.DB, instanceID, policyID int64, status string, backupSize, actualSize int64, completedAt time.Time) int64 {
	t.Helper()
	startedAt := completedAt.Add(-5 * time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policyID,
		TriggerSource:   model.BackupTriggerSourceScheduled,
		Type:            "rolling",
		Status:          status,
		SnapshotPath:    "/backups/" + itoa(policyID),
		BackupSizeBytes: backupSize,
		ActualSizeBytes: actualSize,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: int64(completedAt.Sub(startedAt).Seconds()),
		ErrorMessage:    "",
		RsyncStats:      "{}",
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	return backup.ID
}
