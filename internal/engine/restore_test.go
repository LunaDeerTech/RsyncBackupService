package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	backupcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
)

func TestRestoreExecutorExecuteRollingSourceUsesDelete(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	snapshotRoot := t.TempDir()
	instance, policy, target, _, _ := createRollingFixtures(t, db, sourceRoot, t.TempDir())
	snapshotPath := filepath.Join(snapshotRoot, "snapshot")
	mustWriteFile(t, filepath.Join(snapshotPath, "alpha.txt"), "alpha-v2")
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", snapshotPath, time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC))
	task := createRestoreTask(t, db, instance.ID, backup.ID, "source", "")

	var calledConfig RsyncConfig
	executor := NewRestoreExecutor(nil, db, t.TempDir())
	executor.now = func() time.Time { return time.Date(2026, 4, 7, 17, 5, 0, 0, time.UTC) }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		calledConfig = cfg
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 100, Remaining: "0:00:00"})
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	if err := executor.Execute(context.Background(), task, backup, &RestoreRequest{RestoreType: "source"}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calledConfig.DisableDelete {
		t.Fatal("DisableDelete = true, want false for source restore")
	}
	assertFileContent(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v2")
	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "success" || loadedTask.CurrentStep != restoreTaskDoneStep {
		t.Fatalf("task = %+v, want successful restore completion", loadedTask)
	}
	if target.ID == 0 {
		t.Fatal("target fixture not populated")
	}
}

func TestRestoreExecutorExecuteRollingCustomDisablesDelete(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	snapshotPath := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(snapshotPath, "alpha.txt"), "alpha-v3")
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", snapshotPath, time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC))
	restoreTarget := t.TempDir()
	task := createRestoreTask(t, db, instance.ID, backup.ID, "custom", restoreTarget)

	var calledConfig RsyncConfig
	executor := NewRestoreExecutor(nil, db, t.TempDir())
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		calledConfig = cfg
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	if err := executor.Execute(context.Background(), task, backup, &RestoreRequest{RestoreType: "custom", TargetPath: restoreTarget}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !calledConfig.DisableDelete {
		t.Fatal("DisableDelete = false, want true for custom restore")
	}
	assertFileContent(t, filepath.Join(restoreTarget, "alpha.txt"), "alpha-v3")
}

func TestRestoreExecutorExecuteColdMergeDecryptExtractAndRestore(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	payloadA := make([]byte, 2*1024*1024)
	payloadB := make([]byte, 2*1024*1024)
	for index := range payloadA {
		payloadA[index] = byte(index % 251)
	}
	for index := range payloadB {
		payloadB[index] = byte((index + 17) % 241)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "alpha.bin"), payloadA, 0o600); err != nil {
		t.Fatalf("WriteFile(alpha.bin) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "beta.bin"), payloadB, 0o600); err != nil {
		t.Fatalf("WriteFile(beta.bin) error = %v", err)
	}

	instance, policy, _, backup, backupTask := createColdFixtures(t, db, sourceRoot, targetRoot)
	hash := backupcrypto.HashEncryptionKey("Cold#123")
	policy.Compression = true
	policy.Encryption = true
	policy.EncryptionKeyHash = &hash
	policy.SplitEnabled = true
	splitSizeMB := 1
	policy.SplitSizeMB = &splitSizeMB
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	backupExecutor := NewColdBackupExecutor(nil, db, t.TempDir())
	backupExecutor.now = func() time.Time { return time.Date(2026, 4, 7, 19, 0, 0, 0, time.UTC) }
	backupExecutor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}
	if err := backupExecutor.Execute(WithColdBackupEncryptionKey(context.Background(), "Cold#123"), backupTask, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "cold-target",
		BackupType:   "cold",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil); err != nil {
		t.Fatalf("Cold backup Execute() error = %v", err)
	}
	backup, err := db.GetBackupByID(backup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}

	restoreTarget := t.TempDir()
	restoreTask := createRestoreTask(t, db, instance.ID, backup.ID, "custom", restoreTarget)
	restoreExecutor := NewRestoreExecutor(nil, db, t.TempDir())
	restoreExecutor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 100, Remaining: "0:00:00"})
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	if err := restoreExecutor.Execute(context.Background(), restoreTask, backup, &RestoreRequest{
		RestoreType:   "custom",
		TargetPath:    restoreTarget,
		EncryptionKey: "Cold#123",
	}, nil); err != nil {
		t.Fatalf("Restore Execute() error = %v", err)
	}
	assertFileBytes(t, filepath.Join(restoreTarget, "alpha.bin"), payloadA)
	assertFileBytes(t, filepath.Join(restoreTarget, "beta.bin"), payloadB)
	loadedTask, err := db.GetTaskByID(restoreTask.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(restore) error = %v", err)
	}
	if loadedTask.Status != "success" {
		t.Fatalf("restore task status = %q, want success", loadedTask.Status)
	}
}

func TestWorkerPoolProcessRestoreTaskKeepsBackupStatus(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", filepath.Join(t.TempDir(), "snapshot"), time.Date(2026, 4, 7, 20, 0, 0, 0, time.UTC))
	task := createRestoreTask(t, db, instance.ID, backup.ID, "source", "")
	queue := NewTaskQueue(1, db)
	workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
	workerPool.restore = restoreTaskExecutorFunc(func(ctx context.Context, task *model.Task, backup *model.Backup, request *RestoreRequest, progressCb func(ProgressInfo)) error {
		completedAt := time.Date(2026, 4, 7, 20, 1, 0, 0, time.UTC)
		task.Status = "success"
		task.Progress = 100
		task.CurrentStep = restoreTaskDoneStep
		task.CompletedAt = &completedAt
		return db.UpdateTask(task)
	})

	if err := workerPool.processTask(context.Background(), task); err != nil {
		t.Fatalf("processTask() error = %v", err)
	}
	loadedBackup, err := db.GetBackupByID(backup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if loadedBackup.Status != "success" {
		t.Fatalf("backup.Status = %q, want success", loadedBackup.Status)
	}
	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "success" {
		t.Fatalf("task.Status = %q, want success", loadedTask.Status)
	}
}

type restoreTaskExecutorFunc func(context.Context, *model.Task, *model.Backup, *RestoreRequest, func(ProgressInfo)) error

func (fn restoreTaskExecutorFunc) Execute(ctx context.Context, task *model.Task, backup *model.Backup, request *RestoreRequest, progressCb func(ProgressInfo)) error {
	return fn(ctx, task, backup, request, progressCb)
}

func createRestoreTask(t *testing.T, db interface{ CreateTask(*model.Task) error }, instanceID, backupID int64, restoreType, targetPath string) *model.Task {
	t.Helper()
	backupIDCopy := backupID
	task := &model.Task{
		InstanceID:   instanceID,
		BackupID:     &backupIDCopy,
		Type:         "restore",
		RestoreType:  restoreType,
		TargetPath:   targetPath,
		Status:       "queued",
		Progress:     0,
		CurrentStep:  "queued",
		ErrorMessage: "",
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask(restore) error = %v", err)
	}
	return task
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("file %q content mismatch", path)
	}
}

func TestRestoreTaskJSONFieldsRoundTrip(t *testing.T) {
	task := model.Task{RestoreType: "custom", TargetPath: "/restore"}
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(payload) == "{}" {
		t.Fatal("task JSON unexpectedly omitted restore fields")
	}
}