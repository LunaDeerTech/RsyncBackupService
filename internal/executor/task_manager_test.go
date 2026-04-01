package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskManagerRejectsDuplicateLockKey(t *testing.T) {
	manager := NewTaskManager()
	lockKey := BuildTaskLockKey(1, 2)

	task, ok := manager.TryStart(lockKey, func() {})
	if !ok {
		t.Fatal("expected first task to start")
	}
	if task.ID == "" {
		t.Fatal("expected task id to be assigned")
	}
	if task.LockKey != lockKey {
		t.Fatalf("expected lock key %q, got %q", lockKey, task.LockKey)
	}
	if task.StartedAt.IsZero() {
		t.Fatal("expected started_at to be set")
	}

	if _, ok := manager.TryStart(lockKey, func() {}); ok {
		t.Fatal("expected second task with duplicate lock key to be rejected")
	}

	manager.Finish(task.ID)

	if _, ok := manager.TryStart(lockKey, func() {}); !ok {
		t.Fatal("expected lock key to be reusable after finish")
	}
}

func TestTaskManagerCancelInvokesHandle(t *testing.T) {
	manager := NewTaskManager()
	ctx, cancel := context.WithCancel(context.Background())

	task, ok := manager.TryStart(BuildTaskLockKey(1, 3), cancel)
	if !ok {
		t.Fatal("expected task to start")
	}

	if err := manager.Cancel(task.ID); err != nil {
		t.Fatalf("cancel task: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected task context to be cancelled")
	}

	if err := manager.Cancel("missing-task"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}
