package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

type runningTaskState struct {
	taskID int64
	cancel context.CancelFunc
}

type TaskQueue struct {
	ch        chan *model.Task
	db        *store.DB
	mu        sync.Mutex
	running   map[int64]runningTaskState
	scheduled map[int64]struct{}
	coldKeys  map[int64]string
}

func NewTaskQueue(bufferSize int, db *store.DB) *TaskQueue {
	if bufferSize <= 0 {
		bufferSize = 1
	}

	return &TaskQueue{
		ch:        make(chan *model.Task, bufferSize),
		db:        db,
		running:   make(map[int64]runningTaskState),
		scheduled: make(map[int64]struct{}),
		coldKeys:  make(map[int64]string),
	}
}

func (q *TaskQueue) Enqueue(task *model.Task) error {
	if q == nil {
		return fmt.Errorf("task queue is nil")
	}
	if q.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	if task.ID <= 0 {
		return fmt.Errorf("task id is required")
	}

	loadedTask, err := q.db.GetTaskByID(task.ID)
	if err != nil {
		return err
	}
	if loadedTask.Status != "queued" {
		return nil
	}

	q.mu.Lock()
	if _, exists := q.scheduled[loadedTask.ID]; exists {
		q.mu.Unlock()
		return nil
	}
	if running, exists := q.running[loadedTask.InstanceID]; exists && running.taskID != loadedTask.ID {
		q.mu.Unlock()
		return nil
	}
	q.mu.Unlock()

	hasRunning, err := q.db.HasRunningTask(loadedTask.InstanceID)
	if err != nil {
		return err
	}
	if hasRunning {
		return nil
	}

	queuedTasks, err := q.db.GetQueuedTasksByInstance(loadedTask.InstanceID)
	if err != nil {
		return err
	}
	if len(queuedTasks) == 0 || queuedTasks[0].ID != loadedTask.ID {
		return nil
	}

	q.mu.Lock()
	if _, exists := q.scheduled[loadedTask.ID]; exists {
		q.mu.Unlock()
		return nil
	}
	if running, exists := q.running[loadedTask.InstanceID]; exists && running.taskID != loadedTask.ID {
		q.mu.Unlock()
		return nil
	}
	q.scheduled[loadedTask.ID] = struct{}{}
	q.mu.Unlock()

	taskCopy := *loadedTask
	go func() {
		q.ch <- &taskCopy
	}()

	return nil
}

func (q *TaskQueue) Dequeue(ctx context.Context) (*model.Task, error) {
	if q == nil {
		return nil, fmt.Errorf("task queue is nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case task := <-q.ch:
		if task == nil {
			return nil, nil
		}

		q.mu.Lock()
		delete(q.scheduled, task.ID)
		q.mu.Unlock()

		return task, nil
	}
}

func (q *TaskQueue) Cancel(taskID int64) error {
	if q == nil {
		return fmt.Errorf("task queue is nil")
	}
	if q.db == nil {
		return fmt.Errorf("database unavailable")
	}

	task, err := q.db.GetTaskByID(taskID)
	if err != nil {
		return err
	}

	switch task.Status {
	case "queued":
		return q.cancelQueuedTask(task)
	case "running":
		q.mu.Lock()
		running, exists := q.running[task.InstanceID]
		q.mu.Unlock()
		if !exists || running.taskID != task.ID || running.cancel == nil {
			return fmt.Errorf("task %d is running but is not managed by the queue", task.ID)
		}

		running.cancel()
		return nil
	default:
		return fmt.Errorf("task %d cannot be cancelled from status %q", task.ID, task.Status)
	}
}

func (q *TaskQueue) OnTaskComplete(instanceID int64) {
	if q == nil || q.db == nil || instanceID <= 0 {
		return
	}

	q.mu.Lock()
	delete(q.running, instanceID)
	q.mu.Unlock()

	queuedTasks, err := q.db.GetQueuedTasksByInstance(instanceID)
	if err != nil || len(queuedTasks) == 0 {
		return
	}

	_ = q.Enqueue(&queuedTasks[0])
}

func (q *TaskQueue) Recover() error {
	if q == nil {
		return fmt.Errorf("task queue is nil")
	}
	if q.db == nil {
		return fmt.Errorf("database unavailable")
	}

	activeTasks, err := q.db.ListActiveTasks()
	if err != nil {
		return err
	}

	interruptedAt := time.Now().UTC()
	const interruptedMessage = "task interrupted by service restart"

	for index := range activeTasks {
		task := activeTasks[index]
		if task.Status != "running" {
			continue
		}

		task.Status = "failed"
		task.CompletedAt = &interruptedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = interruptedMessage
		if err := q.db.UpdateTask(&task); err != nil {
			return err
		}

		if task.BackupID != nil {
			backup, err := q.db.GetBackupByID(*task.BackupID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err == nil {
				backup.Status = "failed"
				backup.CompletedAt = &interruptedAt
				backup.DurationSeconds = elapsedSeconds(backup.StartedAt, interruptedAt)
				backup.ErrorMessage = interruptedMessage
				if err := q.db.UpdateBackup(backup); err != nil {
					return err
				}
			}
		}

		if err := q.db.UpdateInstanceStatus(task.InstanceID, "idle"); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	for index := range activeTasks {
		if activeTasks[index].Status != "queued" {
			continue
		}
		if err := q.Enqueue(&activeTasks[index]); err != nil {
			return err
		}
	}

	return nil
}

func (q *TaskQueue) SetColdEncryptionKey(taskID int64, key string) {
	if q == nil || taskID <= 0 {
		return
	}

	trimmed := strings.TrimSpace(key)
	q.mu.Lock()
	defer q.mu.Unlock()

	if trimmed == "" {
		delete(q.coldKeys, taskID)
		return
	}

	q.coldKeys[taskID] = trimmed
}

func (q *TaskQueue) coldEncryptionKey(taskID int64) string {
	if q == nil || taskID <= 0 {
		return ""
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	return q.coldKeys[taskID]
}

func (q *TaskQueue) clearTaskRuntimeData(taskID int64) {
	if q == nil || taskID <= 0 {
		return
	}

	q.mu.Lock()
	delete(q.coldKeys, taskID)
	delete(q.scheduled, taskID)
	q.mu.Unlock()
}

func (q *TaskQueue) beginTask(task *model.Task, cancel context.CancelFunc) (bool, error) {
	if q == nil {
		return false, fmt.Errorf("task queue is nil")
	}
	if q.db == nil {
		return false, fmt.Errorf("database unavailable")
	}
	if task == nil {
		return false, fmt.Errorf("task is nil")
	}

	q.mu.Lock()
	if running, exists := q.running[task.InstanceID]; exists && running.taskID != task.ID {
		q.mu.Unlock()
		return false, nil
	}
	q.running[task.InstanceID] = runningTaskState{taskID: task.ID, cancel: cancel}
	q.mu.Unlock()

	hasRunning, err := q.db.HasRunningTask(task.InstanceID)
	if err != nil {
		q.releaseTask(task.InstanceID, task.ID)
		return false, err
	}
	if hasRunning {
		q.releaseTask(task.InstanceID, task.ID)
		return false, nil
	}

	return true, nil
}

func (q *TaskQueue) releaseTask(instanceID, taskID int64) {
	if q == nil || instanceID <= 0 || taskID <= 0 {
		return
	}

	q.mu.Lock()
	if running, exists := q.running[instanceID]; exists && running.taskID == taskID {
		delete(q.running, instanceID)
	}
	q.mu.Unlock()
}

func (q *TaskQueue) cancelQueuedTask(task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	completedAt := time.Now().UTC()
	task.Status = "cancelled"
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = "task cancelled"
	if err := q.db.UpdateTask(task); err != nil {
		return err
	}

	if task.BackupID != nil {
		backup, err := q.db.GetBackupByID(*task.BackupID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			backup.Status = "cancelled"
			backup.CompletedAt = &completedAt
			backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
			backup.ErrorMessage = "task cancelled"
			if err := q.db.UpdateBackup(backup); err != nil {
				return err
			}
		}
	}

	q.clearTaskRuntimeData(task.ID)
	return nil
}