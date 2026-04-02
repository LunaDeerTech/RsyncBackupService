package executor

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type RunningTask struct {
	ID              string
	LockKey         string
	InstanceID      uint
	StorageTargetID uint
	StartedAt       time.Time
	Cancel          context.CancelFunc
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
	instanceID, storageTargetID, err := ParseTaskLockKey(lockKey)
	if err != nil {
		instanceID = 0
		storageTargetID = 0
	}
	task := RunningTask{
		ID:              taskID,
		LockKey:         lockKey,
		InstanceID:      instanceID,
		StorageTargetID: storageTargetID,
		StartedAt:       time.Now().UTC(),
		Cancel:          cancel,
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

func (m *TaskManager) List() []RunningTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks := make([]RunningTask, 0, len(m.tasksByID))
	for _, task := range m.tasksByID {
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].StartedAt.Equal(tasks[j].StartedAt) {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].StartedAt.Before(tasks[j].StartedAt)
	})

	return tasks
}
