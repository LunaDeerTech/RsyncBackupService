package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestRollingBackupExecutorAllocateSnapshotPathsAddsCollisionSuffix(t *testing.T) {
	targetRoot := t.TempDir()
	instanceName := "mysql-prod"
	baseTime := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	collidingPath := filepath.Join(targetRoot, instanceName, baseTime.Format("20060102-150405"))
	if err := os.MkdirAll(collidingPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(collision) error = %v", err)
	}

	executor := &RollingBackupExecutor{
		now: func() time.Time { return baseTime },
	}

	snapshotPath, latestPath, err := executor.allocateSnapshotPaths(context.Background(), &model.BackupTarget{
		StorageType: "local",
		StoragePath: targetRoot,
	}, nil, instanceName)
	if err != nil {
		t.Fatalf("allocateSnapshotPaths() error = %v", err)
	}

	wantSnapshot := filepath.Join(targetRoot, instanceName, "20260407-120000-01")
	if snapshotPath != wantSnapshot {
		t.Fatalf("snapshotPath = %q, want %q", snapshotPath, wantSnapshot)
	}
	wantLatest := filepath.Join(targetRoot, instanceName, "latest")
	if latestPath != wantLatest {
		t.Fatalf("latestPath = %q, want %q", latestPath, wantLatest)
	}
}

func TestRollingBackupExecutorExecuteLocalToLocalCreatesLatestAndRecordsSuccess(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")
	mustWriteFile(t, filepath.Join(sourceRoot, "nested", "beta.txt"), "beta-v1")

	instance, policy, _, pendingBackup, task := createRollingFixtures(t, db, sourceRoot, targetRoot)
	baseTime := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	var progressEvents []ProgressInfo
	executor := NewRollingBackupExecutor(nil, db)
	executor.dataDir = t.TempDir()
	executor.now = func() time.Time { return baseTime }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		if cfg.LinkDestPath != "" {
			t.Fatalf("LinkDestPath = %q, want empty on first backup", cfg.LinkDestPath)
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 40, Remaining: "0:00:05"})
			loadedTask, err := db.GetTaskByID(task.ID)
			if err != nil {
				t.Fatalf("GetTaskByID(progress) error = %v", err)
			}
			if loadedTask.Progress != 40 {
				t.Fatalf("task progress during rsync = %d, want %d", loadedTask.Progress, 40)
			}
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
		Name:         "rolling-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, func(progress ProgressInfo) {
		progressEvents = append(progressEvents, progress)
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(progressEvents) != 2 {
		t.Fatalf("progressEvents len = %d, want %d", len(progressEvents), 2)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	wantSnapshot := filepath.Join(targetRoot, instance.Name, "20260407-120000")
	if backup.Status != "success" {
		t.Fatalf("backup.Status = %q, want success", backup.Status)
	}
	if backup.SnapshotPath != wantSnapshot {
		t.Fatalf("backup.SnapshotPath = %q, want %q", backup.SnapshotPath, wantSnapshot)
	}
	if backup.BackupSizeBytes != int64(len("alpha-v1")+len("beta-v1")) {
		t.Fatalf("backup.BackupSizeBytes = %d, want %d", backup.BackupSizeBytes, len("alpha-v1")+len("beta-v1"))
	}
	if backup.ActualSizeBytes != backup.BackupSizeBytes {
		t.Fatalf("backup.ActualSizeBytes = %d, want %d", backup.ActualSizeBytes, backup.BackupSizeBytes)
	}
	if !strings.Contains(backup.RsyncStats, "TransferSize") {
		t.Fatalf("backup.RsyncStats = %q, want marshaled rsync stats", backup.RsyncStats)
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(final) error = %v", err)
	}
	if loadedTask.Status != "success" {
		t.Fatalf("task.Status = %q, want success", loadedTask.Status)
	}
	if loadedTask.Progress != 100 {
		t.Fatalf("task.Progress = %d, want 100", loadedTask.Progress)
	}
	if loadedTask.CurrentStep != rollingTaskDoneStep {
		t.Fatalf("task.CurrentStep = %q, want %q", loadedTask.CurrentStep, rollingTaskDoneStep)
	}

	latestPath := filepath.Join(targetRoot, instance.Name, "latest")
	resolvedLatest, err := os.Readlink(latestPath)
	if err != nil {
		t.Fatalf("Readlink(latest) error = %v", err)
	}
	if resolvedLatest != wantSnapshot {
		t.Fatalf("latest symlink = %q, want %q", resolvedLatest, wantSnapshot)
	}
	assertFileContent(t, filepath.Join(wantSnapshot, "alpha.txt"), "alpha-v1")
	assertFileContent(t, filepath.Join(wantSnapshot, "nested", "beta.txt"), "beta-v1")
}

func TestRollingBackupExecutorExecuteLocalToLocalUsesLinkDestForUnchangedFiles(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")
	mustWriteFile(t, filepath.Join(sourceRoot, "beta.txt"), "beta-v1")

	instance, policy, _, firstBackup, firstTask := createRollingFixtures(t, db, sourceRoot, targetRoot)
	firstTime := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	firstExecutor := NewRollingBackupExecutor(nil, db)
	firstExecutor.dataDir = t.TempDir()
	firstExecutor.now = func() time.Time { return firstTime }
	firstExecutor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 100, Remaining: "0:00:00"})
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}
	if err := firstExecutor.Execute(context.Background(), firstTask, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "rolling-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	firstLoadedBackup, err := db.GetBackupByID(firstBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID(first) error = %v", err)
	}

	mustWriteFile(t, filepath.Join(sourceRoot, "beta.txt"), "beta-v2")
	secondBackup, secondTask, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun(second) error = %v", err)
	}

	var secondCall RsyncConfig
	secondTime := time.Date(2026, 4, 7, 12, 1, 0, 0, time.UTC)
	secondExecutor := NewRollingBackupExecutor(nil, db)
	secondExecutor.dataDir = t.TempDir()
	secondExecutor.now = func() time.Time { return secondTime }
	secondExecutor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		secondCall = cfg
		stats, err := emulateLocalSnapshotRsync(cfg)
		if err != nil {
			return nil, err
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 100, Remaining: "0:00:00"})
		}
		return &RsyncResult{ExitCode: 0, Stats: stats}, nil
	}

	if err := secondExecutor.Execute(context.Background(), secondTask, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "rolling-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil); err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if secondCall.LinkDestPath != firstLoadedBackup.SnapshotPath {
		t.Fatalf("second LinkDestPath = %q, want %q", secondCall.LinkDestPath, firstLoadedBackup.SnapshotPath)
	}

	secondLoadedBackup, err := db.GetBackupByID(secondBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID(second) error = %v", err)
	}
	firstAlpha := filepath.Join(firstLoadedBackup.SnapshotPath, "alpha.txt")
	secondAlpha := filepath.Join(secondLoadedBackup.SnapshotPath, "alpha.txt")
	if !sameInode(t, firstAlpha, secondAlpha) {
		t.Fatalf("expected unchanged alpha.txt to be hard-linked between snapshots")
	}
	if sameInode(t, filepath.Join(firstLoadedBackup.SnapshotPath, "beta.txt"), filepath.Join(secondLoadedBackup.SnapshotPath, "beta.txt")) {
		t.Fatalf("expected changed beta.txt to be copied instead of hard-linked")
	}
}

func TestRollingBackupExecutorExecuteMarksBackupFailedOnRsyncError(t *testing.T) {
	db := newRollingTestDB(t)
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "alpha.txt"), "alpha-v1")

	instance, policy, _, pendingBackup, task := createRollingFixtures(t, db, sourceRoot, targetRoot)
	executor := NewRollingBackupExecutor(nil, db)
	executor.dataDir = t.TempDir()
	executor.now = func() time.Time { return time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC) }
	executor.executeRsync = func(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
		return &RsyncResult{ExitCode: 23}, ErrRsyncPartialTransfer
	}

	err := executor.Execute(context.Background(), task, policy, instance, &model.BackupTarget{
		ID:           policy.TargetID,
		Name:         "rolling-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  targetRoot,
		HealthStatus: "healthy",
	}, nil)
	if !errors.Is(err, ErrRsyncPartialTransfer) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrRsyncPartialTransfer)
	}

	backup, err := db.GetBackupByID(pendingBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID(failed) error = %v", err)
	}
	if backup.Status != "failed" {
		t.Fatalf("backup.Status = %q, want failed", backup.Status)
	}
	if !strings.Contains(backup.ErrorMessage, ErrRsyncPartialTransfer.Error()) {
		t.Fatalf("backup.ErrorMessage = %q, want %q", backup.ErrorMessage, ErrRsyncPartialTransfer.Error())
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(failed) error = %v", err)
	}
	if loadedTask.Status != "failed" {
		t.Fatalf("task.Status = %q, want failed", loadedTask.Status)
	}
	if !strings.Contains(loadedTask.ErrorMessage, ErrRsyncPartialTransfer.Error()) {
		t.Fatalf("task.ErrorMessage = %q, want %q", loadedTask.ErrorMessage, ErrRsyncPartialTransfer.Error())
	}
}

func newRollingTestDB(t *testing.T) *store.DB {
	t.Helper()

	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}

	return db
}

func createRollingFixtures(t *testing.T, db *store.DB, sourceRoot, targetRoot string) (*model.Instance, *model.Policy, *model.BackupTarget, *model.Backup, *model.Task) {
	t.Helper()

	instance := &model.Instance{
		Name:       "mysql-prod",
		SourceType: "local",
		SourcePath: sourceRoot,
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "rolling-target",
		BackupType:    "rolling",
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
		Name:           "hourly-rolling",
		Type:           "rolling",
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

func emulateLocalSnapshotRsync(cfg RsyncConfig) (RsyncStats, error) {
	if normalizeRsyncType(cfg.SourceType) != "local" || normalizeRsyncType(cfg.DestType) != "local" {
		return RsyncStats{}, fmt.Errorf("test stub only supports local rsync")
	}

	var stats RsyncStats
	err := filepath.Walk(cfg.SourcePath, func(sourcePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(cfg.SourcePath, sourcePath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(cfg.DestPath, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		stats.TotalFiles++
		stats.TotalSize += info.Size()

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		if cfg.LinkDestPath != "" {
			linkDestFile := filepath.Join(cfg.LinkDestPath, relPath)
			match, err := filesMatch(sourcePath, linkDestFile)
			if err != nil {
				return err
			}
			if match {
				if err := os.Link(linkDestFile, destPath); err != nil {
					return err
				}
				return nil
			}
		}

		if err := copyFile(sourcePath, destPath); err != nil {
			return err
		}
		stats.TransferFiles++
		stats.TransferSize += info.Size()
		return nil
	})
	if err != nil {
		return RsyncStats{}, err
	}
	stats.Speed = "1 bytes/sec"
	return stats, nil
}

func filesMatch(leftPath, rightPath string) (bool, error) {
	leftData, err := os.ReadFile(leftPath)
	if err != nil {
		return false, err
	}
	rightData, err := os.ReadFile(rightPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return string(leftData) == string(rightData), nil
}

func copyFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func mustWriteFile(t *testing.T, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(filePath), err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filePath, err)
	}
}

func assertFileContent(t *testing.T, filePath, want string) {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", filePath, err)
	}
	if string(data) != want {
		t.Fatalf("file content = %q, want %q", string(data), want)
	}
}

func sameInode(t *testing.T, leftPath, rightPath string) bool {
	t.Helper()
	leftInfo, err := os.Stat(leftPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", leftPath, err)
	}
	rightInfo, err := os.Stat(rightPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", rightPath, err)
	}

	leftStat, ok := leftInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("left stat type = %T, want *syscall.Stat_t", leftInfo.Sys())
	}
	rightStat, ok := rightInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("right stat type = %T, want *syscall.Stat_t", rightInfo.Sys())
	}

	return leftStat.Ino == rightStat.Ino && leftStat.Dev == rightStat.Dev
}