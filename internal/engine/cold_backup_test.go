package engine

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"rsync-backup-service/internal/audit"
	backupcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestColdBackupExecutorExecuteDirectoryModeMovesSnapshotAndCleansTemp(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")
	mustWriteFile(t, filepath.Join(sourceRoot, "nested", "beta.txt"), "beta-v1")

	instance, policy, _, pendingBackup, task := createColdFixtures(t, db, sourceRoot, targetRoot)
	baseTime := time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC)

	var progressEvents []ProgressInfo
	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	executor.now = func() time.Time { return baseTime }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 30, Remaining: "0:00:05"})
			progressCb(ProgressInfo{Percentage: 100, Remaining: "0:00:00"})
		}
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	err := executor.Execute(context.Background(), task, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "cold-target",
		BackupType:   "cold",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, func(progress ProgressInfo) {
		progressEvents = append(progressEvents, progress)
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(progressEvents) != 3 {
		t.Fatalf("progressEvents len = %d, want %d", len(progressEvents), 3)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if backup.Status != "success" {
		t.Fatalf("backup.Status = %q, want success", backup.Status)
	}
	storageKey := backupInstanceStorageKey(instance)
	wantSnapshot := filepath.Join(targetRoot, storageKey, "20260407-130000", instance.Name+"-20260407-130000")
	if backup.SnapshotPath != wantSnapshot {
		t.Fatalf("backup.SnapshotPath = %q, want %q", backup.SnapshotPath, wantSnapshot)
	}
	if backup.BackupSizeBytes != int64(len("alpha-v1")+len("beta-v1")) {
		t.Fatalf("backup.BackupSizeBytes = %d, want %d", backup.BackupSizeBytes, len("alpha-v1")+len("beta-v1"))
	}
	if backup.ActualSizeBytes != backup.BackupSizeBytes {
		t.Fatalf("backup.ActualSizeBytes = %d, want %d", backup.ActualSizeBytes, backup.BackupSizeBytes)
	}
	assertFileContent(t, filepath.Join(wantSnapshot, "alpha.txt"), "alpha-v1")
	assertFileContent(t, filepath.Join(wantSnapshot, "nested", "beta.txt"), "beta-v1")

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "success" || loadedTask.CurrentStep != coldTaskDoneStep || loadedTask.Progress != 100 {
		t.Fatalf("task = %+v, want successful completed task", loadedTask)
	}

	tempRoot := filepath.Join(executor.dataDir, "temp", itoa64(task.ID))
	if _, err := os.Stat(tempRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tempRoot stat error = %v, want not exist", err)
	}
}

func TestColdBackupExecutorExecuteCompressedEncryptedSplitStoresParts(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	payloadA := make([]byte, 3*1024*1024)
	payloadB := make([]byte, 3*1024*1024)
	if _, err := rand.Read(payloadA); err != nil {
		t.Fatalf("rand.Read(payloadA) error = %v", err)
	}
	if _, err := rand.Read(payloadB); err != nil {
		t.Fatalf("rand.Read(payloadB) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "alpha.bin"), payloadA, 0o600); err != nil {
		t.Fatalf("WriteFile(alpha.bin) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "beta.bin"), payloadB, 0o600); err != nil {
		t.Fatalf("WriteFile(beta.bin) error = %v", err)
	}

	instance, policy, _, pendingBackup, task := createColdFixtures(t, db, sourceRoot, targetRoot)
	hash := backupcrypto.HashEncryptionKey("Cold#123")
	policy.Compression = true
	policy.Encryption = true
	policy.EncryptionKeyHash = &hash
	policy.SplitEnabled = true
	splitSizeMB := 1
	policy.SplitSizeMB = &splitSizeMB
	baseTime := time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC)

	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	executor.now = func() time.Time { return baseTime }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	ctx := WithColdBackupEncryptionKey(context.Background(), "Cold#123")
	if err := executor.Execute(ctx, task, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "cold-target",
		BackupType:   "cold",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if backup.Status != "success" {
		t.Fatalf("backup.Status = %q, want success", backup.Status)
	}
	storageKey := backupInstanceStorageKey(instance)
	wantPrefix := filepath.Join(targetRoot, storageKey, "20260407-140000", instance.Name+"-20260407-140000.tar.gz.enc.part001")
	if backup.SnapshotPath != wantPrefix {
		t.Fatalf("backup.SnapshotPath = %q, want %q", backup.SnapshotPath, wantPrefix)
	}
	partMatches, err := filepath.Glob(filepath.Join(targetRoot, storageKey, "20260407-140000", instance.Name+"-20260407-140000.tar.gz.enc.part*"))
	if err != nil {
		t.Fatalf("Glob(parts) error = %v", err)
	}
	if len(partMatches) == 0 {
		t.Fatal("want split part files, got none")
	}
	if len(partMatches) < 2 {
		t.Fatalf("split parts len = %d, want at least 2", len(partMatches))
	}
	var totalSize int64
	for _, partPath := range partMatches {
		info, err := os.Stat(partPath)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", partPath, err)
		}
		totalSize += info.Size()
	}
	if backup.BackupSizeBytes != totalSize {
		t.Fatalf("backup.BackupSizeBytes = %d, want %d", backup.BackupSizeBytes, totalSize)
	}
	if backup.ActualSizeBytes <= 0 {
		t.Fatalf("backup.ActualSizeBytes = %d, want > 0", backup.ActualSizeBytes)
	}
}

func TestColdBackupExecutorExecuteFailsOnWrongEncryptionKey(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")

	instance, policy, _, pendingBackup, task := createColdFixtures(t, db, sourceRoot, targetRoot)
	hash := backupcrypto.HashEncryptionKey("Cold#123")
	policy.Encryption = true
	policy.EncryptionKeyHash = &hash
	policy.Compression = true

	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	executor.now = func() time.Time { return time.Date(2026, 4, 7, 15, 0, 0, 0, time.UTC) }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	err := executor.Execute(WithColdBackupEncryptionKey(context.Background(), "Wrong#123"), task, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "cold-target",
		BackupType:   "cold",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not match policy") {
		t.Fatalf("Execute() error = %v, want encryption key mismatch", err)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if backup.Status != "failed" {
		t.Fatalf("backup.Status = %q, want failed", backup.Status)
	}
}

func TestColdBackupExecutorExecuteMarksCancelledOnContextCancel(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")

	instance, policy, _, pendingBackup, task := createColdFixtures(t, db, sourceRoot, targetRoot)
	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	executor.now = func() time.Time { return time.Date(2026, 4, 7, 16, 0, 0, 0, time.UTC) }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		return &RsyncResult{ExitCode: 130}, context.Canceled
	}

	err := executor.Execute(context.Background(), task, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "cold-target",
		BackupType:   "cold",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if backup.Status != "cancelled" {
		t.Fatalf("backup.Status = %q, want cancelled", backup.Status)
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "cancelled" {
		t.Fatalf("task.Status = %q, want cancelled", loadedTask.Status)
	}
}

func TestColdBackupExecutorDeliverArtifactsLocalRetriesFailedEntry(t *testing.T) {
	db := newRollingTestDB(t)
	_, policy, target, _, task := createColdFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetryEnabled = true
	policy.RetryMaxRetries = 2
	target.StorageType = "local"

	stageDir := t.TempDir()
	runDir := filepath.Join(t.TempDir(), "run")
	mustWriteFile(t, filepath.Join(stageDir, "alpha.txt"), "alpha")
	mustWriteFile(t, filepath.Join(stageDir, "beta.txt"), "beta")

	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	attempts := map[string]int{}
	var delays []time.Duration
	executor.movePath = func(sourcePath, destPath string) error {
		entry := filepath.Base(sourcePath)
		attempts[entry]++
		if entry == "alpha.txt" && attempts[entry] == 1 {
			return errors.New("transient local move error")
		}
		return movePath(sourcePath, destPath)
	}
	executor.sleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	_, err := executor.deliverArtifacts(context.Background(), policy, target, nil, runDir, stageDir, []string{"alpha.txt", "beta.txt"}, task, nil)
	if err != nil {
		t.Fatalf("deliverArtifacts() error = %v", err)
	}
	if attempts["alpha.txt"] != 2 {
		t.Fatalf("alpha attempts = %d, want 2", attempts["alpha.txt"])
	}
	if attempts["beta.txt"] != 1 {
		t.Fatalf("beta attempts = %d, want 1", attempts["beta.txt"])
	}
	if len(delays) != 1 || delays[0] != managedRetryDelay(1) {
		t.Fatalf("retry delays = %v, want [%s]", delays, managedRetryDelay(1))
	}
	assertFileContent(t, filepath.Join(runDir, "alpha.txt"), "alpha")
	assertFileContent(t, filepath.Join(runDir, "beta.txt"), "beta")
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetry, 1)
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetryExhausted, 0)

	detail := loadAuditDetailByAction(t, db, audit.ActionBackupMoveRetry)
	if detail["entry"] != "alpha.txt" {
		t.Fatalf("retry audit entry = %v, want alpha.txt", detail["entry"])
	}
	if detail["operation"] != "local_move" {
		t.Fatalf("retry audit operation = %v, want local_move", detail["operation"])
	}
	if detail["next_delay"] != managedRetryDelay(1).String() {
		t.Fatalf("retry audit next_delay = %v, want %s", detail["next_delay"], managedRetryDelay(1))
	}
}

func TestColdBackupExecutorDeliverArtifactsOpenListRetriesOnlyFailedEntry(t *testing.T) {
	db := newRollingTestDB(t)
	_, policy, target, _, task := createColdFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetryEnabled = true
	policy.RetryMaxRetries = 2
	target.StorageType = "openlist"

	stageDir := t.TempDir()
	mustWriteFile(t, filepath.Join(stageDir, "alpha.tar"), "alpha")
	mustWriteFile(t, filepath.Join(stageDir, "beta.tar"), "beta")

	session := &fakeColdBackupOpenListSession{
		remainingFailures: map[string]int{"/remote/run/alpha.tar": 1},
		uploadAttempts:    map[string]int{},
	}
	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	var delays []time.Duration
	executor.openListSessionFactory = func(ctx context.Context, remote *model.RemoteConfig) (coldBackupOpenListSession, error) {
		return session, nil
	}
	executor.sleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	_, err := executor.deliverArtifacts(context.Background(), policy, target, nil, "/remote/run", stageDir, []string{"alpha.tar", "beta.tar"}, task, nil)
	if err != nil {
		t.Fatalf("deliverArtifacts() error = %v", err)
	}
	if len(session.ensureDirs) != 1 || session.ensureDirs[0] != "/remote/run" {
		t.Fatalf("ensureDirs = %v, want [/remote/run]", session.ensureDirs)
	}
	if session.uploadAttempts["/remote/run/alpha.tar"] != 2 {
		t.Fatalf("alpha upload attempts = %d, want 2", session.uploadAttempts["/remote/run/alpha.tar"])
	}
	if session.uploadAttempts["/remote/run/beta.tar"] != 1 {
		t.Fatalf("beta upload attempts = %d, want 1", session.uploadAttempts["/remote/run/beta.tar"])
	}
	if len(delays) != 1 || delays[0] != managedRetryDelay(1) {
		t.Fatalf("retry delays = %v, want [%s]", delays, managedRetryDelay(1))
	}
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetry, 1)
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetryExhausted, 0)

	detail := loadAuditDetailByAction(t, db, audit.ActionBackupMoveRetry)
	if detail["entry"] != "alpha.tar" {
		t.Fatalf("retry audit entry = %v, want alpha.tar", detail["entry"])
	}
	if detail["operation"] != "openlist_upload" {
		t.Fatalf("retry audit operation = %v, want openlist_upload", detail["operation"])
	}
	if detail["dest_path"] != "/remote/run/alpha.tar" {
		t.Fatalf("retry audit dest_path = %v, want /remote/run/alpha.tar", detail["dest_path"])
	}
}

func TestColdBackupExecutorDeliverArtifactsOpenListFailsAfterRetryExhausted(t *testing.T) {
	db := newRollingTestDB(t)
	_, policy, target, _, task := createColdFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetryEnabled = true
	policy.RetryMaxRetries = 2
	target.StorageType = "openlist"

	stageDir := t.TempDir()
	mustWriteFile(t, filepath.Join(stageDir, "alpha.tar"), "alpha")

	session := &fakeColdBackupOpenListSession{
		remainingFailures: map[string]int{"/remote/run/alpha.tar": 3},
		uploadAttempts:    map[string]int{},
	}
	executor := NewColdBackupExecutor(nil, db, t.TempDir())
	var delays []time.Duration
	executor.openListSessionFactory = func(ctx context.Context, remote *model.RemoteConfig) (coldBackupOpenListSession, error) {
		return session, nil
	}
	executor.sleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	_, err := executor.deliverArtifacts(context.Background(), policy, target, nil, "/remote/run", stageDir, []string{"alpha.tar"}, task, nil)
	if err == nil || !strings.Contains(err.Error(), "delivery failed after 2 retries") {
		t.Fatalf("deliverArtifacts() error = %v, want exhausted retry failure", err)
	}
	if session.uploadAttempts["/remote/run/alpha.tar"] != 3 {
		t.Fatalf("alpha upload attempts = %d, want 3", session.uploadAttempts["/remote/run/alpha.tar"])
	}
	if len(delays) != 2 || delays[0] != managedRetryDelay(1) || delays[1] != managedRetryDelay(2) {
		t.Fatalf("retry delays = %v, want [%s %s]", delays, managedRetryDelay(1), managedRetryDelay(2))
	}
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetry, 2)
	assertEngineAuditCount(t, db, audit.ActionBackupMoveRetryExhausted, 1)

	detail := loadAuditDetailByAction(t, db, audit.ActionBackupMoveRetryExhausted)
	if detail["entry"] != "alpha.tar" {
		t.Fatalf("exhausted audit entry = %v, want alpha.tar", detail["entry"])
	}
	if detail["operation"] != "openlist_upload" {
		t.Fatalf("exhausted audit operation = %v, want openlist_upload", detail["operation"])
	}
	if detail["final"] != true {
		t.Fatalf("exhausted audit final = %v, want true", detail["final"])
	}
}

func createColdFixtures(t *testing.T, db *store.DB, sourceRoot, targetRoot string) (*model.Instance, *model.Policy, *model.BackupTarget, *model.Backup, *model.Task) {
	t.Helper()

	instance := &model.Instance{
		Name:            "mysql-prod",
		SourceType:      "local",
		SourcePath:      sourceRoot,
		ExcludePatterns: []string{"*.tmp", "cache/**"},
		Status:          "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   targetRoot,
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policy := &model.Policy{
		InstanceID:     instance.ID,
		Name:           "nightly-cold",
		Type:           "cold",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	backup, task, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun() error = %v", err)
	}

	return instance, policy, target, backup, task
}

func itoa64(value int64) string {
	return strconv.FormatInt(value, 10)
}

type fakeColdBackupOpenListSession struct {
	ensureDirs        []string
	remainingFailures map[string]int
	uploadAttempts    map[string]int
}

func (s *fakeColdBackupOpenListSession) EnsureDir(ctx context.Context, remotePath string) error {
	s.ensureDirs = append(s.ensureDirs, remotePath)
	return nil
}

func (s *fakeColdBackupOpenListSession) UploadFile(ctx context.Context, localPath, remotePath string) error {
	if s.uploadAttempts == nil {
		s.uploadAttempts = map[string]int{}
	}
	s.uploadAttempts[remotePath]++
	if s.remainingFailures[remotePath] > 0 {
		s.remainingFailures[remotePath]--
		return errors.New("transient remote upload error")
	}
	return nil
}

func loadAuditDetailByAction(t *testing.T, db *store.DB, action string) map[string]any {
	t.Helper()

	var raw string
	if err := db.QueryRow(`SELECT detail FROM audit_logs WHERE action = ? ORDER BY id LIMIT 1`, action).Scan(&raw); err != nil {
		t.Fatalf("query audit detail action %q error = %v", action, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		t.Fatalf("json.Unmarshal(detail) error = %v", err)
	}
	return detail
}
