package executor

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type RunningTask struct {
	ID        string
	LockKey   string
	StartedAt time.Time
	Cancel    context.CancelFunc
}

type TaskManager struct {
	nextID      uint64
	mu          sync.Mutex
	tasksByID   map[string]RunningTask
	lockKeyToID map[string]string
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasksByID:   make(map[string]RunningTask),
		lockKeyToID: make(map[string]string),
	}
}

func (m *TaskManager) TryStart(lockKey string, cancel context.CancelFunc) (RunningTask, bool) {
	if cancel == nil {
		cancel = func() {}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.lockKeyToID[lockKey]; exists {
		return RunningTask{}, false
	}

	taskID := strconv.FormatUint(atomic.AddUint64(&m.nextID, 1), 10)
	task := RunningTask{
		ID:        taskID,
		LockKey:   lockKey,
		StartedAt: time.Now().UTC(),
		Cancel:    cancel,
	}
	m.tasksByID[task.ID] = task
	m.lockKeyToID[lockKey] = task.ID

	return task, true
}

func (m *TaskManager) Finish(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasksByID[taskID]
	if !exists {
		return
	}

	delete(m.tasksByID, taskID)
	delete(m.lockKeyToID, task.LockKey)
}

func (m *TaskManager) Cancel(taskID string) error {
	m.mu.Lock()
	task, exists := m.tasksByID[taskID]
	m.mu.Unlock()
	if !exists {
		return ErrTaskNotFound
	}

	task.Cancel()
	return nil
}
