package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

type backupTaskExecutor interface {
	Execute(context.Context, *model.Task, *model.Policy, *model.Instance, *model.BackupTarget, func(ProgressInfo)) error
}

type WorkerPool struct {
	workers int
	queue   *TaskQueue
	rolling backupTaskExecutor
	cold    backupTaskExecutor
	db      *store.DB
}

func NewWorkerPool(workers int, queue *TaskQueue, rolling *RollingBackupExecutor, cold *ColdBackupExecutor, db *store.DB) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if db == nil && queue != nil {
		db = queue.db
	}
	if rolling == nil && db != nil {
		rolling = NewRollingBackupExecutor(nil, db)
	}
	if cold == nil && db != nil {
		cold = NewColdBackupExecutor(nil, db, resolveRollingDataDir())
	}

	return &WorkerPool{
		workers: workers,
		queue:   queue,
		rolling: rolling,
		cold:    cold,
		db:      db,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	if wp == nil || wp.queue == nil || wp.db == nil {
		return
	}

	var once sync.Once
	stopWorkers := func() {
		once.Do(func() {
			slog.Info("worker pool stopped")
		})
	}

	for workerID := 0; workerID < wp.workers; workerID++ {
		go func(id int) {
			for {
				task, err := wp.queue.Dequeue(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
						stopWorkers()
						return
					}
					slog.Error("worker dequeue failed", "worker", id, "error", err)
					continue
				}
				if task == nil {
					continue
				}

				if err := wp.processTask(ctx, task); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("worker task execution failed", "worker", id, "task_id", task.ID, "error", err)
				}
			}
		}(workerID + 1)
	}
}

func (wp *WorkerPool) processTask(ctx context.Context, task *model.Task) error {
	if wp == nil {
		return fmt.Errorf("worker pool is nil")
	}
	if wp.queue == nil {
		return fmt.Errorf("task queue is nil")
	}
	if wp.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	loadedTask, err := wp.db.GetTaskByID(task.ID)
	if err != nil {
		return err
	}
	if loadedTask.Status != "queued" {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	started, err := wp.queue.beginTask(loadedTask, cancel)
	if err != nil {
		cancel()
		return err
	}
	if !started {
		cancel()
		return wp.queue.Enqueue(loadedTask)
	}
	defer cancel()
	defer wp.queue.clearTaskRuntimeData(loadedTask.ID)
	defer wp.queue.OnTaskComplete(loadedTask.InstanceID)

	backup, policy, instance, target, err := wp.loadTaskContext(loadedTask)
	if backup != nil {
		defer wp.queue.reloadScheduledPolicy(backup)
	}
	if err != nil {
		return wp.failTask(loadedTask, backup, err)
	}

	normalizedType := normalizeTaskType(loadedTask.Type, policy)
	if normalizedType != loadedTask.Type {
		loadedTask.Type = normalizedType
		if err := wp.db.UpdateTask(loadedTask); err != nil {
			return wp.failTask(loadedTask, backup, err)
		}
	}

	if err := wp.db.UpdateInstanceStatus(instance.ID, "running"); err != nil {
		return wp.failTask(loadedTask, backup, err)
	}
	defer func() {
		if err := wp.db.UpdateInstanceStatus(instance.ID, "idle"); err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("restore instance status failed", "instance_id", instance.ID, "task_id", loadedTask.ID, "error", err)
		}
	}()

	switch loadedTask.Type {
	case "rolling":
		if wp.rolling == nil {
			return wp.failTask(loadedTask, backup, fmt.Errorf("rolling executor is unavailable"))
		}
		return wp.rolling.Execute(runCtx, loadedTask, policy, instance, target, nil)
	case "cold":
		if wp.cold == nil {
			return wp.failTask(loadedTask, backup, fmt.Errorf("cold executor is unavailable"))
		}
		if policy.Encryption {
			runCtx = WithColdBackupEncryptionKey(runCtx, wp.queue.coldEncryptionKey(loadedTask.ID))
		}
		return wp.cold.Execute(runCtx, loadedTask, policy, instance, target, nil)
	default:
		return wp.failTask(loadedTask, backup, fmt.Errorf("unsupported task type %q", loadedTask.Type))
	}
}

func (wp *WorkerPool) loadTaskContext(task *model.Task) (*model.Backup, *model.Policy, *model.Instance, *model.BackupTarget, error) {
	if task == nil {
		return nil, nil, nil, nil, fmt.Errorf("task is nil")
	}

	instance, err := wp.db.GetInstanceByID(task.InstanceID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if task.BackupID == nil {
		return nil, nil, instance, nil, fmt.Errorf("task %d is missing backup id", task.ID)
	}

	backup, err := wp.db.GetBackupByID(*task.BackupID)
	if err != nil {
		return nil, nil, instance, nil, err
	}

	policy, err := wp.db.GetPolicyByID(backup.PolicyID)
	if err != nil {
		return backup, nil, instance, nil, err
	}

	target, err := wp.db.GetBackupTargetByID(policy.TargetID)
	if err != nil {
		return backup, policy, instance, nil, err
	}

	return backup, policy, instance, target, nil
}

func (wp *WorkerPool) failTask(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := time.Now().UTC()
	trimmed := strings.TrimSpace(runErr.Error())
	var persistErr error

	if backup != nil {
		backup.Status = "failed"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = trimmed
		if err := wp.db.UpdateBackup(backup); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}
	if task != nil {
		task.Status = "failed"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = trimmed
		if err := wp.db.UpdateTask(task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}

	return runErr
}

func normalizeTaskType(taskType string, policy *model.Policy) string {
	trimmed := strings.ToLower(strings.TrimSpace(taskType))
	if trimmed == "backup" && policy != nil {
		return strings.ToLower(strings.TrimSpace(policy.Type))
	}
	return trimmed
}