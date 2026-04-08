package engine

import (
	"context"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestSchedulerNextIntervalTrigger(t *testing.T) {
	baseNow := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	t.Run("first scheduled run waits interval from now", func(t *testing.T) {
		db := newRollingTestDB(t)
		policy := createSchedulerPolicy(t, db, "60")
		scheduler := NewScheduler(db, nil)

		got, shouldSchedule, err := scheduler.nextIntervalTrigger(policy, baseNow)
		if err != nil {
			t.Fatalf("nextIntervalTrigger() error = %v", err)
		}
		if !shouldSchedule {
			t.Fatal("nextIntervalTrigger() shouldSchedule = false, want true")
		}

		want := baseNow.Add(time.Minute)
		if !got.Equal(want) {
			t.Fatalf("nextIntervalTrigger() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})

	t.Run("manual backups do not affect next scheduled run", func(t *testing.T) {
		db := newRollingTestDB(t)
		policy := createSchedulerPolicy(t, db, "60")
		createCompletedPolicyRunWithSource(t, db, policy, model.BackupTriggerSourceManual, baseNow.Add(-30*time.Second))
		scheduler := NewScheduler(db, nil)

		got, shouldSchedule, err := scheduler.nextIntervalTrigger(policy, baseNow)
		if err != nil {
			t.Fatalf("nextIntervalTrigger() error = %v", err)
		}
		if !shouldSchedule {
			t.Fatal("nextIntervalTrigger() shouldSchedule = false, want true")
		}

		want := baseNow.Add(time.Minute)
		if !got.Equal(want) {
			t.Fatalf("nextIntervalTrigger() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})

	t.Run("scheduled completion drives next run", func(t *testing.T) {
		db := newRollingTestDB(t)
		policy := createSchedulerPolicy(t, db, "60")
		createCompletedPolicyRunWithSource(t, db, policy, model.BackupTriggerSourceScheduled, baseNow.Add(-30*time.Second))
		scheduler := NewScheduler(db, nil)

		got, shouldSchedule, err := scheduler.nextIntervalTrigger(policy, baseNow)
		if err != nil {
			t.Fatalf("nextIntervalTrigger() error = %v", err)
		}
		if !shouldSchedule {
			t.Fatal("nextIntervalTrigger() shouldSchedule = false, want true")
		}

		want := baseNow.Add(30 * time.Second)
		if !got.Equal(want) {
			t.Fatalf("nextIntervalTrigger() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})

	t.Run("overdue schedule triggers immediately", func(t *testing.T) {
		db := newRollingTestDB(t)
		policy := createSchedulerPolicy(t, db, "60")
		createCompletedPolicyRunWithSource(t, db, policy, model.BackupTriggerSourceScheduled, baseNow.Add(-2*time.Minute))
		scheduler := NewScheduler(db, nil)

		got, shouldSchedule, err := scheduler.nextIntervalTrigger(policy, baseNow)
		if err != nil {
			t.Fatalf("nextIntervalTrigger() error = %v", err)
		}
		if !shouldSchedule {
			t.Fatal("nextIntervalTrigger() shouldSchedule = false, want true")
		}
		if !got.Equal(baseNow) {
			t.Fatalf("nextIntervalTrigger() = %s, want %s", got.Format(time.RFC3339), baseNow.Format(time.RFC3339))
		}
	})

	t.Run("active scheduled run suppresses timer creation until completion", func(t *testing.T) {
		db := newRollingTestDB(t)
		policy := createSchedulerPolicy(t, db, "60")
		if _, _, err := db.CreatePendingPolicyRunWithSource(policy, model.BackupTriggerSourceScheduled); err != nil {
			t.Fatalf("CreatePendingPolicyRunWithSource() error = %v", err)
		}
		scheduler := NewScheduler(db, nil)

		_, shouldSchedule, err := scheduler.nextIntervalTrigger(policy, baseNow)
		if err != nil {
			t.Fatalf("nextIntervalTrigger() error = %v", err)
		}
		if shouldSchedule {
			t.Fatal("nextIntervalTrigger() shouldSchedule = true, want false")
		}
	})
}

func TestSchedulerHandleTimerCreatesScheduledTask(t *testing.T) {
	db := newRollingTestDB(t)
	policy := createSchedulerPolicy(t, db, "60")
	queue := NewTaskQueue(4, db)
	scheduler := NewScheduler(db, queue)

	scheduler.handleTimer(policy.ID, 0)

	dequeueCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	task, err := queue.Dequeue(dequeueCtx)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if task == nil {
		t.Fatal("Dequeue() = nil, want scheduled task")
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "queued" {
		t.Fatalf("task status = %q, want queued", loadedTask.Status)
	}
	if loadedTask.BackupID == nil {
		t.Fatal("task.BackupID = nil, want linked backup")
	}

	backup, err := db.GetBackupByID(*loadedTask.BackupID)
	if err != nil {
		t.Fatalf("GetBackupByID() error = %v", err)
	}
	if backup.TriggerSource != model.BackupTriggerSourceScheduled {
		t.Fatalf("backup.TriggerSource = %q, want %q", backup.TriggerSource, model.BackupTriggerSourceScheduled)
	}
	if backup.Status != "pending" {
		t.Fatalf("backup status = %q, want pending", backup.Status)
	}
}

func TestSchedulerGetUpcomingTasks(t *testing.T) {
	baseNow := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	db := newRollingTestDB(t)

	first := createSchedulerPolicyWithNames(t, db, "scheduler-first", "policy-first", "60")
	second := createSchedulerPolicyWithNames(t, db, "scheduler-second", "policy-second", "120")
	third := createSchedulerPolicyWithNames(t, db, "scheduler-third", "policy-third", "300")
	fourth := createSchedulerPolicyWithNames(t, db, "scheduler-fourth", "policy-fourth", "7200")

	createCompletedPolicyRunWithSource(t, db, first, model.BackupTriggerSourceScheduled, baseNow.Add(-30*time.Second))
	createCompletedPolicyRunWithSource(t, db, second, model.BackupTriggerSourceScheduled, baseNow.Add(-10*time.Second))
	createCompletedPolicyRunWithSource(t, db, third, model.BackupTriggerSourceScheduled, baseNow.Add(-10*time.Minute))
	createCompletedPolicyRunWithSource(t, db, fourth, model.BackupTriggerSourceScheduled, baseNow.Add(-30*time.Minute))

	scheduler := NewScheduler(db, nil)
	scheduler.SetClock(func() time.Time { return baseNow })

	upcoming := scheduler.GetUpcomingTasks(2 * time.Minute)
	if len(upcoming) != 3 {
		t.Fatalf("len(GetUpcomingTasks()) = %d, want 3", len(upcoming))
	}
	if upcoming[0].PolicyID != third.ID || !upcoming[0].NextRunAt.Equal(baseNow) {
		t.Fatalf("upcoming[0] = %+v, want overdue third policy at now", upcoming[0])
	}
	if upcoming[1].PolicyID != first.ID || !upcoming[1].NextRunAt.Equal(baseNow.Add(30*time.Second)) {
		t.Fatalf("upcoming[1] = %+v, want first policy at %s", upcoming[1], baseNow.Add(30*time.Second).Format(time.RFC3339))
	}
	if upcoming[2].PolicyID != second.ID || !upcoming[2].NextRunAt.Equal(baseNow.Add(110*time.Second)) {
		t.Fatalf("upcoming[2] = %+v, want second policy at %s", upcoming[2], baseNow.Add(110*time.Second).Format(time.RFC3339))
	}
	for _, task := range upcoming {
		if task.InstanceName == "" {
			t.Fatalf("InstanceName = empty for upcoming task %+v", task)
		}
	}
}

func createSchedulerPolicyWithNames(t *testing.T, db *store.DB, instanceName, policyName, intervalSeconds string) *model.Policy {
	t.Helper()

	instance := &model.Instance{
		Name:       instanceName,
		SourceType: "local",
		SourcePath: t.TempDir(),
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          policyName + "-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policy := &model.Policy{
		InstanceID:     instance.ID,
		Name:           policyName,
		Type:           "rolling",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  intervalSeconds,
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	return policy
}

func createSchedulerPolicy(t *testing.T, db *store.DB, intervalSeconds string) *model.Policy {
	t.Helper()

	instance := &model.Instance{
		Name:       "mysql-prod",
		SourceType: "local",
		SourcePath: t.TempDir(),
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "rolling-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policy := &model.Policy{
		InstanceID:     instance.ID,
		Name:           "scheduled-rolling",
		Type:           "rolling",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  intervalSeconds,
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	return policy
}

func createCompletedPolicyRunWithSource(t *testing.T, db *store.DB, policy *model.Policy, triggerSource string, completedAt time.Time) {
	t.Helper()

	backup, task, err := db.CreatePendingPolicyRunWithSource(policy, triggerSource)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRunWithSource() error = %v", err)
	}

	backup.Status = "success"
	backup.CompletedAt = &completedAt
	if err := db.UpdateBackup(backup); err != nil {
		t.Fatalf("UpdateBackup() error = %v", err)
	}

	task.Status = "success"
	task.CompletedAt = &completedAt
	task.CurrentStep = "done"
	if err := db.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}
}
