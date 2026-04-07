package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
)

func TestWorkerPoolProcessTaskCreatesBackupFailedRiskEvent(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	_, task, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun() error = %v", err)
	}

	queue := NewTaskQueue(1, db)
	workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
	workerPool.SetRiskDetector(NewRiskDetector(db, nil, audit.NewLogger(db)))
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
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed, model.RiskSeverityWarning)
}
