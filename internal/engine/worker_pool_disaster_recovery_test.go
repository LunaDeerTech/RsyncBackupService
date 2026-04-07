package engine

import (
	"context"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
)

func TestWorkerPoolInvalidatesDisasterRecoveryCacheAfterBackup(t *testing.T) {
	db := newRollingTestDB(t)
	instance, _, _, _, task := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	disasterRecovery := service.NewDisasterRecoveryService(db)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	disasterRecovery.SetClock(func() time.Time { return now })
	before, err := disasterRecovery.GetScore(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetScore(before) error = %v", err)
	}

	queue := NewTaskQueue(1, db)
	workerPool := NewWorkerPool(1, queue, nil, nil, db, nil)
	workerPool.SetDisasterRecoveryService(disasterRecovery)
	workerPool.rolling = backupTaskExecutorFunc(func(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
		backup, err := db.GetBackupByID(*task.BackupID)
		if err != nil {
			return err
		}
		completedAt := now.Add(10 * time.Minute)
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

	disasterRecovery.SetClock(func() time.Time { return now.Add(1 * time.Minute) })
	after, err := disasterRecovery.GetScore(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetScore(after) error = %v", err)
	}
	if after.Total <= before.Total {
		t.Fatalf("after.Total = %v, want greater than before.Total = %v", after.Total, before.Total)
	}
}
