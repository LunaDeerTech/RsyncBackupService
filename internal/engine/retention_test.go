package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/store"
)

func TestRetentionCleanerCleanByCountDeletesOldRollingBackupsAndRepairsLatest(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetentionType = "count"
	policy.RetentionValue = 1
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	oldestTime := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)
	middleTime := oldestTime.Add(24 * time.Hour)
	latestTime := middleTime.Add(24 * time.Hour)
	storageKey := backupInstanceStorageKey(instance)

	oldest := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260404-080000"), oldestTime)
	middle := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260405-080000"), middleTime)
	latest := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260406-080000"), latestTime)

	mustMkdirAll(t, oldest.SnapshotPath)
	mustMkdirAll(t, middle.SnapshotPath)
	mustMkdirAll(t, latest.SnapshotPath)
	latestLinkPath := filepath.Join(target.StoragePath, storageKey, "latest")
	mustMkdirAll(t, filepath.Dir(latestLinkPath))
	if err := os.Symlink(oldest.SnapshotPath, latestLinkPath); err != nil {
		t.Fatalf("Symlink(old latest) error = %v", err)
	}

	cleaner := NewRetentionCleaner(db, t.TempDir())
	cleaner.now = func() time.Time { return latestTime.Add(time.Hour) }
	if err := cleaner.CleanByPolicy(context.Background(), policy); err != nil {
		t.Fatalf("CleanByPolicy(count) error = %v", err)
	}

	assertBackupRemoved(t, db, oldest.ID)
	assertBackupRemoved(t, db, middle.ID)
	assertTaskRowsForBackup(t, db, oldest.ID, 0)
	assertTaskRowsForBackup(t, db, middle.ID, 0)
	assertBackupExists(t, db, latest.ID)
	if _, err := os.Stat(oldest.SnapshotPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(oldest snapshot) error = %v, want not exist", err)
	}
	if _, err := os.Stat(middle.SnapshotPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(middle snapshot) error = %v, want not exist", err)
	}
	resolvedLatest, err := os.Readlink(latestLinkPath)
	if err != nil {
		t.Fatalf("Readlink(latest) error = %v", err)
	}
	if resolvedLatest != latest.SnapshotPath {
		t.Fatalf("latest link = %q, want %q", resolvedLatest, latest.SnapshotPath)
	}
}

func TestRetentionCleanerCleanByTimeDeletesExpiredColdSplitBackupAndKeepsLastSuccess(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.Type = "cold"
	policy.RetentionType = "time"
	policy.RetentionValue = 1
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}
	target.BackupType = "cold"
	if err := db.UpdateBackupTarget(target); err != nil {
		t.Fatalf("UpdateBackupTarget() error = %v", err)
	}

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	storageKey := backupInstanceStorageKey(instance)
	expiredRunDir := filepath.Join(target.StoragePath, storageKey, "20260405-010000")
	expiredPart1 := filepath.Join(expiredRunDir, "mysql-prod-20260405-010000.tar.gz.enc.part001")
	expiredPart2 := filepath.Join(expiredRunDir, "mysql-prod-20260405-010000.tar.gz.enc.part002")
	keptRunDir := filepath.Join(target.StoragePath, storageKey, "20260407-010000")
	keptFile := filepath.Join(keptRunDir, "mysql-prod-20260407-010000.tar.gz.enc")

	expired := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "cold", expiredPart1, now.Add(-48*time.Hour))
	kept := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "cold", keptFile, now.Add(-2*time.Hour))

	mustWriteFile(t, expiredPart1, "part-1")
	mustWriteFile(t, expiredPart2, "part-2")
	mustWriteFile(t, keptFile, "latest-cold")

	cleaner := NewRetentionCleaner(db, t.TempDir())
	cleaner.now = func() time.Time { return now }
	if err := cleaner.CleanByPolicy(context.Background(), policy); err != nil {
		t.Fatalf("CleanByPolicy(time) error = %v", err)
	}

	assertBackupRemoved(t, db, expired.ID)
	assertTaskRowsForBackup(t, db, expired.ID, 0)
	assertBackupExists(t, db, kept.ID)
	if _, err := os.Stat(expiredPart1); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(expired part1) error = %v, want not exist", err)
	}
	if _, err := os.Stat(expiredPart2); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(expired part2) error = %v, want not exist", err)
	}
	if _, err := os.Stat(keptFile); err != nil {
		t.Fatalf("Stat(kept file) error = %v", err)
	}
}

func TestRetentionCleanerOpenListSplitCleanupRefreshesExpiredToken(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createColdFixtures(t, db, t.TempDir(), t.TempDir())

	encodedConfig, err := openlist.EncodeStoredConfig("secret", "")
	if err != nil {
		t.Fatalf("EncodeStoredConfig() error = %v", err)
	}
	provider := "openlist"
	remote := &model.RemoteConfig{
		Name:          "openlist-retention",
		Type:          "openlist",
		Username:      "admin",
		CloudProvider: &provider,
		CloudConfig:   encodedConfig,
	}

	loginCalls := 0
	removeCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			loginCalls++
			token := "token-1"
			if loginCalls == 2 {
				token = "token-2"
			}
			writeRetentionOpenListJSON(t, w, http.StatusOK, map[string]any{
				"code":    http.StatusOK,
				"message": "success",
				"data":    map[string]any{"token": token},
			})
		case "/api/fs/remove":
			removeCalls++
			var payload struct {
				Dir   string   `json:"dir"`
				Names []string `json:"names"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode remove payload error = %v", err)
			}
			switch removeCalls {
			case 1:
				if got := r.Header.Get("Authorization"); got != "token-1" {
					t.Fatalf("first remove Authorization = %q, want token-1", got)
				}
				if len(payload.Names) != 1 || payload.Names[0] != "artifact.tar.gz.part001" {
					t.Fatalf("first remove names = %v", payload.Names)
				}
				writeRetentionOpenListJSON(t, w, http.StatusOK, map[string]any{"code": http.StatusOK, "message": "success", "data": nil})
			case 2:
				if got := r.Header.Get("Authorization"); got != "token-1" {
					t.Fatalf("expired remove Authorization = %q, want token-1", got)
				}
				writeRetentionOpenListJSON(t, w, http.StatusUnauthorized, map[string]any{
					"code":    http.StatusUnauthorized,
					"message": "token is expired",
					"data":    nil,
				})
			case 3:
				if got := r.Header.Get("Authorization"); got != "token-2" {
					t.Fatalf("retried remove Authorization = %q, want token-2", got)
				}
				if len(payload.Names) != 1 || payload.Names[0] != "artifact.tar.gz.part002" {
					t.Fatalf("retried remove names = %v", payload.Names)
				}
				writeRetentionOpenListJSON(t, w, http.StatusOK, map[string]any{"code": http.StatusOK, "message": "success", "data": nil})
			default:
				if got := r.Header.Get("Authorization"); got != "token-2" {
					t.Fatalf("later remove Authorization = %q, want token-2", got)
				}
				writeRetentionOpenListJSON(t, w, http.StatusOK, map[string]any{"code": http.StatusNotFound, "message": "object not found", "data": nil})
			}
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	remote.Host = server.URL
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}
	target.StorageType = "openlist"
	target.StoragePath = "/mnt/baidu/RBS"
	target.RemoteConfigID = &remote.ID
	if err := db.UpdateBackupTarget(target); err != nil {
		t.Fatalf("UpdateBackupTarget() error = %v", err)
	}
	policy.RetentionType = "count"
	policy.RetentionValue = 1
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	oldPart := "/mnt/baidu/RBS/3/20260414-143125/artifact.tar.gz.part001"
	oldBackup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "cold", oldPart, now.Add(-48*time.Hour))
	latest := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "cold", "/mnt/baidu/RBS/3/20260729-000000/latest.tar.gz", now.Add(-time.Hour))

	cleaner := NewRetentionCleaner(db, t.TempDir())
	if err := cleaner.CleanByPolicy(context.Background(), policy); err != nil {
		t.Fatalf("CleanByPolicy(openlist split) error = %v", err)
	}

	assertBackupRemoved(t, db, oldBackup.ID)
	assertTaskRowsForBackup(t, db, oldBackup.ID, 0)
	assertBackupExists(t, db, latest.ID)
	if loginCalls != 2 {
		t.Fatalf("login calls = %d, want 2", loginCalls)
	}
	if removeCalls != 5 {
		t.Fatalf("remove calls = %d, want 5", removeCalls)
	}
	assertAuditLogCount(t, db, instance.ID, "backup.cleanup_failed", 0)
}

func writeRetentionOpenListJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode OpenList response error = %v", err)
	}
}

func TestRetentionCleanerContinuesWhenArtifactDeletionFailsAndWritesAuditLog(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetentionType = "count"
	policy.RetentionValue = 1
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	storageKey := backupInstanceStorageKey(instance)
	first := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260404-010000"), now.Add(-72*time.Hour))
	second := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260405-010000"), now.Add(-48*time.Hour))
	latest := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260406-010000"), now.Add(-24*time.Hour))
	mustMkdirAll(t, first.SnapshotPath)
	mustMkdirAll(t, second.SnapshotPath)
	mustMkdirAll(t, latest.SnapshotPath)

	cleaner := NewRetentionCleaner(db, t.TempDir())
	cleaner.now = func() time.Time { return now }
	cleaner.removeAll = func(path string) error {
		if path == first.SnapshotPath {
			return errors.New("simulated remove failure")
		}
		return os.RemoveAll(path)
	}

	err := cleaner.CleanByPolicy(context.Background(), policy)
	if err == nil {
		t.Fatal("CleanByPolicy() error = nil, want aggregated error")
	}
	assertBackupExists(t, db, first.ID)
	assertBackupRemoved(t, db, second.ID)
	assertBackupExists(t, db, latest.ID)
	assertAuditLogCount(t, db, instance.ID, "backup.cleanup_failed", 1)
}

func TestRetentionCleanerDeleteBackupRemovesLastBackupAndLatestLink(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	storageKey := backupInstanceStorageKey(instance)
	backupPath := filepath.Join(target.StoragePath, storageKey, "20260407-010000")
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", backupPath, time.Date(2026, 4, 7, 1, 0, 0, 0, time.UTC))
	mustMkdirAll(t, backupPath)

	latestLinkPath := filepath.Join(target.StoragePath, storageKey, "latest")
	mustMkdirAll(t, filepath.Dir(latestLinkPath))
	if err := os.Symlink(backupPath, latestLinkPath); err != nil {
		t.Fatalf("Symlink(latest) error = %v", err)
	}

	cleaner := NewRetentionCleaner(db, t.TempDir())
	if err := cleaner.DeleteBackup(context.Background(), backup.ID); err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	assertBackupRemoved(t, db, backup.ID)
	assertTaskRowsForBackup(t, db, backup.ID, 0)
	if _, err := os.Stat(backupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(backup) error = %v, want not exist", err)
	}
	if _, err := os.Lstat(latestLinkPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(latest) error = %v, want not exist", err)
	}
}

func TestRetentionCleanerDeleteBackupKeepsRowsWhenArtifactDeletionFails(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	backupPath := filepath.Join(target.StoragePath, backupInstanceStorageKey(instance), "20260407-010000")
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", backupPath, time.Date(2026, 4, 7, 1, 0, 0, 0, time.UTC))
	mustMkdirAll(t, backupPath)

	cleaner := NewRetentionCleaner(db, t.TempDir())
	cleaner.removeAll = func(path string) error {
		return errors.New("simulated remove failure")
	}
	if err := cleaner.DeleteBackup(context.Background(), backup.ID); err == nil {
		t.Fatal("DeleteBackup() error = nil, want artifact deletion error")
	}

	assertBackupExists(t, db, backup.ID)
	assertTaskRowsForBackup(t, db, backup.ID, 1)
	assertAuditLogCount(t, db, instance.ID, "backup.cleanup_failed", 1)
}

func TestWorkerPoolProcessTaskTriggersRetentionAfterSuccess(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, target, _, task := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetentionType = "count"
	policy.RetentionValue = 1
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	storageKey := backupInstanceStorageKey(instance)
	oldBackup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(target.StoragePath, storageKey, "20260406-010000"), now.Add(-time.Hour))
	mustMkdirAll(t, oldBackup.SnapshotPath)

	queue := NewTaskQueue(1, db)
	cleaner := NewRetentionCleaner(db, t.TempDir())
	cleaner.now = func() time.Time { return now }
	fake := backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
		backup, err := db.GetBackupByID(*task.BackupID)
		if err != nil {
			return err
		}
		completedAt := now
		backup.Type = policy.Type
		backup.Status = "success"
		backup.SnapshotPath = filepath.Join(target.StoragePath, backupInstanceStorageKey(instance), "20260407-120000")
		backup.CompletedAt = &completedAt
		if err := db.UpdateBackup(backup); err != nil {
			return err
		}
		if err := os.MkdirAll(backup.SnapshotPath, 0o755); err != nil {
			return err
		}
		task.Status = "success"
		task.Progress = 100
		task.CurrentStep = rollingTaskDoneStep
		task.CompletedAt = &completedAt
		return db.UpdateTask(task)
	})

	workerPool := NewWorkerPool(1, queue, nil, nil, db, cleaner)
	workerPool.rolling = fake
	workerPool.cold = fake

	if err := workerPool.processTask(context.Background(), task); err != nil {
		t.Fatalf("processTask() error = %v", err)
	}

	assertBackupRemoved(t, db, oldBackup.ID)
	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "success" {
		t.Fatalf("loadedTask.Status = %q, want success", loadedTask.Status)
	}
}

type backupTaskExecutorFunc func(context.Context, *model.Task, *model.Policy, *model.Instance, *model.BackupTarget, func(ProgressInfo)) error

func (fn backupTaskExecutorFunc) Execute(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
	return fn(ctx, task, policy, instance, target, progressCb)
}

func insertSuccessBackupWithTask(t *testing.T, db *store.DB, instanceID, policyID int64, backupType, snapshotPath string, completedAt time.Time) *model.Backup {
	t.Helper()

	startedAt := completedAt.Add(-10 * time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policyID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            backupType,
		Status:          "success",
		SnapshotPath:    snapshotPath,
		BackupSizeBytes: 128,
		ActualSizeBytes: 128,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: int64(completedAt.Sub(startedAt).Seconds()),
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	backupID := backup.ID
	task := &model.Task{
		InstanceID:   instanceID,
		BackupID:     &backupID,
		Type:         backupType,
		Status:       "success",
		Progress:     100,
		CurrentStep:  "completed",
		StartedAt:    &startedAt,
		CompletedAt:  &completedAt,
		ErrorMessage: "",
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	return backup
}

func assertBackupRemoved(t *testing.T, db *store.DB, backupID int64) {
	t.Helper()
	if _, err := db.GetBackupByID(backupID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetBackupByID(%d) error = %v, want sql.ErrNoRows", backupID, err)
	}
}

func assertBackupExists(t *testing.T, db *store.DB, backupID int64) {
	t.Helper()
	if _, err := db.GetBackupByID(backupID); err != nil {
		t.Fatalf("GetBackupByID(%d) error = %v", backupID, err)
	}
}

func assertTaskRowsForBackup(t *testing.T, db *store.DB, backupID int64, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE backup_id = ?`, backupID).Scan(&got); err != nil {
		t.Fatalf("count tasks for backup %d error = %v", backupID, err)
	}
	if got != want {
		t.Fatalf("task rows for backup %d = %d, want %d", backupID, got, want)
	}
}

func assertAuditLogCount(t *testing.T, db *store.DB, instanceID int64, action string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE instance_id = ? AND action = ?`, instanceID, action).Scan(&got); err != nil {
		t.Fatalf("count audit logs action %q error = %v", action, err)
	}
	if got != want {
		t.Fatalf("audit logs for action %q = %d, want %d", action, got, want)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}
