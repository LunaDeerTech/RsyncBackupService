package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
)

func TestWorkerPoolProcessTaskDoesNotRetryCancelledTask(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, backup, task := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetryEnabled = true
	policy.RetryMaxRetries = 3
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	queue := NewTaskQueue(1, db)
	workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
	workerPool.SetAuditLogger(audit.NewLogger(db))
	workerPool.SetRiskDetector(NewRiskDetector(db, nil, audit.NewLogger(db)))

	retryScheduled := false
	workerPool.afterFunc = func(delay time.Duration, fn func()) *time.Timer {
		retryScheduled = true
		return nil
	}
	workerPool.rolling = backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
		completedAt := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
		backup.Status = "cancelled"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = 5
		backup.ErrorMessage = context.Canceled.Error()
		if err := db.UpdateBackup(backup); err != nil {
			return err
		}
		task.Status = "cancelled"
		task.CompletedAt = &completedAt
		task.ErrorMessage = context.Canceled.Error()
		if err := db.UpdateTask(task); err != nil {
			return err
		}
		return context.Canceled
	})

	err := workerPool.processTask(context.Background(), task)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("processTask() error = %v, want context.Canceled", err)
	}
	if retryScheduled {
		t.Fatal("retry was scheduled for a cancelled task")
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "cancelled" {
		t.Fatalf("task status = %q, want cancelled", loadedTask.Status)
	}

	loadedBackup, err := db.GetBackupByID(backup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if loadedBackup.Status != "cancelled" {
		t.Fatalf("backup status = %q, want cancelled", loadedBackup.Status)
	}

	assertEngineAuditCount(t, db, audit.ActionBackupRetry, 0)
	assertEngineAuditCount(t, db, audit.ActionBackupRetryExhausted, 0)
	assertNoActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed)
}

func TestWorkerPoolProcessTaskReconcilesSuccessfulBackupAfterTaskPersistError(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, backup, task := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	policy.RetryEnabled = true
	policy.RetryMaxRetries = 3
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}

	queue := NewTaskQueue(1, db)
	workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
	workerPool.SetAuditLogger(audit.NewLogger(db))

	retryScheduled := false
	workerPool.afterFunc = func(delay time.Duration, fn func()) *time.Timer {
		retryScheduled = true
		return nil
	}
	workerPool.rolling = backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
		startedAt := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
		completedAt := startedAt.Add(2 * time.Minute)

		backup.Status = "running"
		backup.StartedAt = &startedAt
		if err := db.UpdateBackup(backup); err != nil {
			return err
		}

		task.Status = "running"
		task.StartedAt = &startedAt
		task.CurrentStep = "syncing"
		if err := db.UpdateTask(task); err != nil {
			return err
		}

		backup.Status = "success"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = 120
		backup.ErrorMessage = ""
		if err := db.UpdateBackup(backup); err != nil {
			return err
		}

		task.Status = "success"
		task.Progress = 100
		task.CurrentStep = rollingTaskDoneStep
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = ""
		return errors.New("update task: simulated transient error")
	})

	if err := workerPool.processTask(context.Background(), task); err != nil {
		t.Fatalf("processTask() error = %v, want reconciled success", err)
	}
	if retryScheduled {
		t.Fatal("retry was scheduled for a backup that already completed successfully")
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "success" {
		t.Fatalf("task status = %q, want success", loadedTask.Status)
	}

	loadedBackup, err := db.GetBackupByID(backup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if loadedBackup.Status != "success" {
		t.Fatalf("backup status = %q, want success", loadedBackup.Status)
	}

	hasRunning, err := db.HasRunningTask(instance.ID)
	if err != nil {
		t.Fatalf("HasRunningTask() error = %v", err)
	}
	if hasRunning {
		t.Fatal("HasRunningTask() = true, want false after reconciliation")
	}

	assertEngineAuditCount(t, db, audit.ActionBackupComplete, 1)
	assertEngineAuditCount(t, db, audit.ActionBackupRetry, 0)
	assertEngineAuditCount(t, db, audit.ActionBackupRetryExhausted, 0)
}
