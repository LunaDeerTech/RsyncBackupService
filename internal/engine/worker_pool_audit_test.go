package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestWorkerPoolWritesBackupCompleteAndFailAuditLogs(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		db := newRollingTestDB(t)
		_, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
		_, task, err := db.CreatePendingPolicyRun(policy)
		if err != nil {
			t.Fatalf("CreatePendingPolicyRun() error = %v", err)
		}

		queue := NewTaskQueue(1, db)
		workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
		workerPool.SetAuditLogger(audit.NewLogger(db))
		workerPool.rolling = backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
			backup, err := db.GetBackupByID(*task.BackupID)
			if err != nil {
				return err
			}
			completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
			backup.Status = "success"
			backup.CompletedAt = &completedAt
			backup.DurationSeconds = 120
			if err := db.UpdateBackup(backup); err != nil {
				return err
			}
			task.Status = "success"
			task.Progress = 100
			task.CurrentStep = "completed"
			task.CompletedAt = &completedAt
			return db.UpdateTask(task)
		})

		if err := workerPool.processTask(context.Background(), task); err != nil {
			t.Fatalf("processTask() error = %v", err)
		}
		assertEngineAuditCount(t, db, audit.ActionBackupComplete, 1)
	})

	t.Run("fail", func(t *testing.T) {
		db := newRollingTestDB(t)
		_, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
		_, task, err := db.CreatePendingPolicyRun(policy)
		if err != nil {
			t.Fatalf("CreatePendingPolicyRun() error = %v", err)
		}

		queue := NewTaskQueue(1, db)
		workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
		workerPool.SetAuditLogger(audit.NewLogger(db))
		workerPool.rolling = backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
			backup, err := db.GetBackupByID(*task.BackupID)
			if err != nil {
				return err
			}
			completedAt := time.Date(2026, 4, 7, 21, 10, 0, 0, time.UTC)
			backup.Status = "failed"
			backup.CompletedAt = &completedAt
			backup.DurationSeconds = 60
			backup.ErrorMessage = "rsync failed"
			if err := db.UpdateBackup(backup); err != nil {
				return err
			}
			task.Status = "failed"
			task.CompletedAt = &completedAt
			task.ErrorMessage = "rsync failed"
			if err := db.UpdateTask(task); err != nil {
				return err
			}
			return errors.New("rsync failed")
		})

		if err := workerPool.processTask(context.Background(), task); err == nil {
			t.Fatal("processTask() error = nil, want failure")
		}
		assertEngineAuditCount(t, db, audit.ActionBackupFail, 1)
	})
}

func TestWorkerPoolWritesRestoreCompleteAndFailAuditLogs(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		db := newRollingTestDB(t)
		instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
		backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", t.TempDir(), time.Date(2026, 4, 7, 20, 0, 0, 0, time.UTC))
		task := createRestoreTask(t, db, instance.ID, backup.ID, "source", "")
		queue := NewTaskQueue(1, db)
		workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
		workerPool.SetAuditLogger(audit.NewLogger(db))
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
		assertEngineAuditCount(t, db, audit.ActionRestoreComplete, 1)
	})

	t.Run("fail", func(t *testing.T) {
		db := newRollingTestDB(t)
		instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
		backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", t.TempDir(), time.Date(2026, 4, 7, 20, 0, 0, 0, time.UTC))
		task := createRestoreTask(t, db, instance.ID, backup.ID, "custom", "/restore")
		queue := NewTaskQueue(1, db)
		workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
		workerPool.SetAuditLogger(audit.NewLogger(db))
		workerPool.restore = restoreTaskExecutorFunc(func(ctx context.Context, task *model.Task, backup *model.Backup, request *RestoreRequest, progressCb func(ProgressInfo)) error {
			completedAt := time.Date(2026, 4, 7, 20, 2, 0, 0, time.UTC)
			task.Status = "failed"
			task.CompletedAt = &completedAt
			task.ErrorMessage = "restore failed"
			if err := db.UpdateTask(task); err != nil {
				return err
			}
			return errors.New("restore failed")
		})

		if err := workerPool.processTask(context.Background(), task); err == nil {
			t.Fatal("processTask() error = nil, want failure")
		}
		assertEngineAuditCount(t, db, audit.ActionRestoreFail, 1)
	})
}

func assertEngineAuditCount(t *testing.T, db *store.DB, action string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action = ?`, action).Scan(&got); err != nil {
		t.Fatalf("count audit logs action %q error = %v", action, err)
	}
	if got != want {
		t.Fatalf("audit logs for action %q = %d, want %d", action, got, want)
	}
}
