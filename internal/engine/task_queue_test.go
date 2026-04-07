package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskQueueSerializesTasksPerInstance(t *testing.T) {
	db := newRollingTestDB(t)
	_, policy, _, _, firstTask := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	_, secondTask, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun(second) error = %v", err)
	}

	queue := NewTaskQueue(4, db)
	if err := queue.Enqueue(firstTask); err != nil {
		t.Fatalf("Enqueue(first) error = %v", err)
	}

	dequeueCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dequeuedFirst, err := queue.Dequeue(dequeueCtx)
	if err != nil {
		t.Fatalf("Dequeue(first) error = %v", err)
	}
	if dequeuedFirst.ID != firstTask.ID {
		t.Fatalf("first dequeued task = %d, want %d", dequeuedFirst.ID, firstTask.ID)
	}

	started, err := queue.beginTask(dequeuedFirst, func() {})
	if err != nil {
		t.Fatalf("beginTask(first) error = %v", err)
	}
	if !started {
		t.Fatal("beginTask(first) = false, want true")
	}
	dequeuedFirst.Status = "running"
	if err := db.UpdateTask(dequeuedFirst); err != nil {
		t.Fatalf("UpdateTask(first running) error = %v", err)
	}

	if err := queue.Enqueue(secondTask); err != nil {
		t.Fatalf("Enqueue(second) error = %v", err)
	}

	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer blockedCancel()
	if _, err := queue.Dequeue(blockedCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Dequeue(second while first running) error = %v, want deadline exceeded", err)
	}

	dequeuedFirst.Status = "success"
	if err := db.UpdateTask(dequeuedFirst); err != nil {
		t.Fatalf("UpdateTask(first success) error = %v", err)
	}
	queue.OnTaskComplete(firstTask.InstanceID)

	nextCtx, nextCancel := context.WithTimeout(context.Background(), time.Second)
	defer nextCancel()
	dequeuedSecond, err := queue.Dequeue(nextCtx)
	if err != nil {
		t.Fatalf("Dequeue(second after completion) error = %v", err)
	}
	if dequeuedSecond.ID != secondTask.ID {
		t.Fatalf("second dequeued task = %d, want %d", dequeuedSecond.ID, secondTask.ID)
	}
}

func TestTaskQueueRecoverMarksRunningFailedAndRequeuesQueued(t *testing.T) {
	db := newRollingTestDB(t)
	_, policy, _, runningBackup, runningTask := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	_, queuedTask, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun(queued) error = %v", err)
	}

	now := time.Now().UTC().Add(-time.Minute)
	runningBackup.Status = "running"
	runningBackup.StartedAt = &now
	if err := db.UpdateBackup(runningBackup); err != nil {
		t.Fatalf("UpdateBackup(running) error = %v", err)
	}
	runningTask.Status = "running"
	runningTask.StartedAt = &now
	runningTask.CurrentStep = "syncing"
	if err := db.UpdateTask(runningTask); err != nil {
		t.Fatalf("UpdateTask(running) error = %v", err)
	}

	queue := NewTaskQueue(4, db)
	if err := queue.Recover(); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}

	recoveredTask, err := db.GetTaskByID(runningTask.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(running) error = %v", err)
	}
	if recoveredTask.Status != "failed" {
		t.Fatalf("recovered running task status = %q, want failed", recoveredTask.Status)
	}

	recoveredBackup, err := db.GetBackupByID(runningBackup.ID)
	if err != nil {
		t.Fatalf("GetBackupByID(running) error = %v", err)
	}
	if recoveredBackup.Status != "failed" {
		t.Fatalf("recovered running backup status = %q, want failed", recoveredBackup.Status)
	}

	dequeueCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dequeuedQueued, err := queue.Dequeue(dequeueCtx)
	if err != nil {
		t.Fatalf("Dequeue(recovered queued) error = %v", err)
	}
	if dequeuedQueued.ID != queuedTask.ID {
		t.Fatalf("recovered queued task = %d, want %d", dequeuedQueued.ID, queuedTask.ID)
	}
}