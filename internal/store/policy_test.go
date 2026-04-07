package store

import (
	"database/sql"
	"errors"
	"testing"

	"rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
)

func TestPolicyCRUDSummaryAndTrigger(t *testing.T) {
	db := newTestDB(t)

	instance := &model.Instance{
		Name:       "mysql-prod",
		SourceType: "local",
		SourcePath: "/srv/mysql",
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	hash := crypto.HashEncryptionKey("Key#123")
	policy := &model.Policy{
		InstanceID:        instance.ID,
		Name:              "nightly-cold",
		Type:              "cold",
		TargetID:          target.ID,
		ScheduleType:      "cron",
		ScheduleValue:     "0 1 * * *",
		Enabled:           true,
		Compression:       true,
		Encryption:        true,
		EncryptionKeyHash: &hash,
		SplitEnabled:      true,
		SplitSizeMB:       intPtr(256),
		RetentionType:     "count",
		RetentionValue:    7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}
	if policy.ID == 0 {
		t.Fatal("CreatePolicy() ID = 0, want generated ID")
	}

	loaded, err := db.GetPolicyByID(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicyByID() error = %v", err)
	}
	if loaded.Name != policy.Name || loaded.TargetID != target.ID {
		t.Fatalf("loaded policy = %+v, want created values", loaded)
	}

	policies, err := db.ListPoliciesByInstance(instance.ID)
	if err != nil {
		t.Fatalf("ListPoliciesByInstance() error = %v", err)
	}
	if len(policies) != 1 || policies[0].ID != policy.ID {
		t.Fatalf("ListPoliciesByInstance() = %+v, want policy %d", policies, policy.ID)
	}

	enabled, err := db.ListEnabledPolicies()
	if err != nil {
		t.Fatalf("ListEnabledPolicies() error = %v", err)
	}
	if len(enabled) != 1 || enabled[0].ID != policy.ID {
		t.Fatalf("ListEnabledPolicies() = %+v, want policy %d", enabled, policy.ID)
	}

	policy.ScheduleType = "interval"
	policy.ScheduleValue = "3600"
	policy.RetentionType = "time"
	policy.RetentionValue = 30
	policy.SplitEnabled = false
	policy.SplitSizeMB = nil
	if err := db.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}
	if policy.ScheduleType != "interval" || policy.RetentionType != "time" {
		t.Fatalf("updated policy = %+v, want interval/time settings", policy)
	}

	backup, task, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun() error = %v", err)
	}
	if backup.ID == 0 || task.ID == 0 {
		t.Fatalf("pending run = (%+v, %+v), want persisted backup/task", backup, task)
	}
	if backup.Status != "pending" || task.Status != "queued" {
		t.Fatalf("pending statuses = (%q, %q), want (pending, queued)", backup.Status, task.Status)
	}
	if task.BackupID == nil || *task.BackupID != backup.ID {
		t.Fatalf("task.BackupID = %v, want %d", task.BackupID, backup.ID)
	}
	if task.Type != policy.Type || task.CurrentStep != "queued" {
		t.Fatalf("queued task = %+v, want type %q and queued step", task, policy.Type)
	}

	activeTasks, err := db.ListActiveTasks()
	if err != nil {
		t.Fatalf("ListActiveTasks() error = %v", err)
	}
	if len(activeTasks) != 1 || activeTasks[0].ID != task.ID {
		t.Fatalf("ListActiveTasks() = %+v, want queued task %d", activeTasks, task.ID)
	}

	instanceTasks, err := db.ListTasksByInstance(instance.ID)
	if err != nil {
		t.Fatalf("ListTasksByInstance() error = %v", err)
	}
	if len(instanceTasks) != 1 || instanceTasks[0].ID != task.ID {
		t.Fatalf("ListTasksByInstance() = %+v, want task %d", instanceTasks, task.ID)
	}

	hasRunning, err := db.HasRunningTask(instance.ID)
	if err != nil {
		t.Fatalf("HasRunningTask() error = %v", err)
	}
	if hasRunning {
		t.Fatal("HasRunningTask() = true, want false for queued-only state")
	}

	queuedTasks, err := db.GetQueuedTasksByInstance(instance.ID)
	if err != nil {
		t.Fatalf("GetQueuedTasksByInstance() error = %v", err)
	}
	if len(queuedTasks) != 1 || queuedTasks[0].ID != task.ID {
		t.Fatalf("GetQueuedTasksByInstance() = %+v, want queued task %d", queuedTasks, task.ID)
	}

	summaries, err := db.ListPolicyExecutionSummaries(instance.ID)
	if err != nil {
		t.Fatalf("ListPolicyExecutionSummaries() error = %v", err)
	}
	summary := summaries[policy.ID]
	if summary.LatestBackupID == nil || *summary.LatestBackupID != backup.ID {
		t.Fatalf("summary.LatestBackupID = %v, want %d", summary.LatestBackupID, backup.ID)
	}
	if summary.LastExecutionStatus == nil || *summary.LastExecutionStatus != "pending" {
		t.Fatalf("summary.LastExecutionStatus = %v, want pending", summary.LastExecutionStatus)
	}
	if summary.LastExecutionTime == nil {
		t.Fatal("summary.LastExecutionTime = nil, want non-nil")
	}

	if err := db.DeletePolicy(policy.ID); err != nil {
		t.Fatalf("DeletePolicy() error = %v", err)
	}
	if _, err := db.GetPolicyByID(policy.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetPolicyByID(deleted) error = %v, want sql.ErrNoRows", err)
	}
	assertPolicyRowCount(t, db, `SELECT COUNT(*) FROM backups WHERE policy_id = ?`, policy.ID, 0)
	assertPolicyRowCount(t, db, `SELECT COUNT(*) FROM tasks WHERE backup_id = ?`, backup.ID, 0)
}

func intPtr(value int) *int {
	return &value
}

func assertPolicyRowCount(t *testing.T, db *DB, query string, arg any, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}
	if got != want {
		t.Fatalf("row count for %q = %d, want %d", query, got, want)
	}
}
