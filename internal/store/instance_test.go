package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
)

func TestInstanceCRUDStatsAndDeleteCleanup(t *testing.T) {
	db := newTestDB(t)
	viewer := createStoreTestUser(t, db, "viewer@example.com", "viewer")

	remote := &model.RemoteConfig{
		Name:           "ssh-remote",
		Type:           "ssh",
		Host:           "10.0.0.8",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: filepath.Join(t.TempDir(), "id_rsa"),
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "primary-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	instance := &model.Instance{
		Name:            "mysql-prod",
		SourceType:      "ssh",
		SourcePath:      "/srv/mysql",
		ExcludePatterns: []string{"*.log", "tmp/**", "*.log"},
		RemoteConfigID:  &remote.ID,
		Status:          "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	if instance.ID == 0 {
		t.Fatal("CreateInstance() ID = 0, want generated ID")
	}

	second := &model.Instance{
		Name:       "postgres-prod",
		SourceType: "local",
		SourcePath: "/srv/postgres",
		Status:     "idle",
	}
	if err := db.CreateInstance(second); err != nil {
		t.Fatalf("CreateInstance(second) error = %v", err)
	}

	loadedByName, err := db.GetInstanceByName(instance.Name)
	if err != nil {
		t.Fatalf("GetInstanceByName() error = %v", err)
	}
	if loadedByName.ID != instance.ID {
		t.Fatalf("GetInstanceByName().ID = %d, want %d", loadedByName.ID, instance.ID)
	}
	if len(loadedByName.ExcludePatterns) != 2 || loadedByName.ExcludePatterns[0] != "*.log" || loadedByName.ExcludePatterns[1] != "tmp/**" {
		t.Fatalf("GetInstanceByName().ExcludePatterns = %#v, want normalized patterns", loadedByName.ExcludePatterns)
	}

	page, err := db.ListInstancesPage(10, 0)
	if err != nil {
		t.Fatalf("ListInstancesPage() error = %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("ListInstancesPage() len = %d, want %d", len(page), 2)
	}

	count, err := db.CountInstances()
	if err != nil {
		t.Fatalf("CountInstances() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountInstances() = %d, want %d", count, 2)
	}

	instance.Name = "mysql-main"
	instance.SourcePath = "/data/mysql"
	instance.ExcludePatterns = []string{"node_modules/", "*.tmp"}
	if err := db.UpdateInstance(instance); err != nil {
		t.Fatalf("UpdateInstance() error = %v", err)
	}
	if instance.Name != "mysql-main" {
		t.Fatalf("UpdateInstance().Name = %q, want %q", instance.Name, "mysql-main")
	}
	if len(instance.ExcludePatterns) != 2 || instance.ExcludePatterns[0] != "node_modules/" || instance.ExcludePatterns[1] != "*.tmp" {
		t.Fatalf("UpdateInstance().ExcludePatterns = %#v, want updated patterns", instance.ExcludePatterns)
	}

	if err := db.UpdateInstanceStatus(instance.ID, "running"); err != nil {
		t.Fatalf("UpdateInstanceStatus() error = %v", err)
	}
	updatedStatus, err := db.GetInstanceByID(instance.ID)
	if err != nil {
		t.Fatalf("GetInstanceByID(updated status) error = %v", err)
	}
	if updatedStatus.Status != "running" {
		t.Fatalf("GetInstanceByID().Status = %q, want %q", updatedStatus.Status, "running")
	}

	if err := db.SetInstancePermissions(instance.ID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	visibleToViewer, err := db.ListInstancesByUserPermission(viewer.ID)
	if err != nil {
		t.Fatalf("ListInstancesByUserPermission() error = %v", err)
	}
	if len(visibleToViewer) != 1 || visibleToViewer[0].ID != instance.ID {
		t.Fatalf("ListInstancesByUserPermission() = %+v, want only instance %d", visibleToViewer, instance.ID)
	}

	visibleCount, err := db.CountInstancesByUserPermission(viewer.ID)
	if err != nil {
		t.Fatalf("CountInstancesByUserPermission() error = %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("CountInstancesByUserPermission() = %d, want %d", visibleCount, 1)
	}

	policyID := createStoreTestPolicy(t, db, instance.ID, target.ID, "daily-backup")
	firstBackupID := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 150, 120, "datetime('now', '-1 day')")
	secondBackupID := insertStoreTestBackupWithRetryRoot(t, db, instance.ID, policyID, firstBackupID, "success", 80, 60, "CURRENT_TIMESTAMP")
	thirdBackupID := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 40, 20, "datetime('now', '-2 day')")
	_ = thirdBackupID
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.complete", "CURRENT_TIMESTAMP")
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.fail", "datetime('now', '-2 day')")
	insertStoreTestTask(t, db, instance.ID, secondBackupID)
	insertStoreTestNotificationSubscription(t, db, viewer.ID, instance.ID)
	insertStoreTestAuditLog(t, db, viewer.ID, instance.ID)

	stats, err := db.GetInstanceStats(instance.ID)
	if err != nil {
		t.Fatalf("GetInstanceStats() error = %v", err)
	}
	if stats.BackupCount != 2 {
		t.Fatalf("stats.BackupCount = %d, want %d", stats.BackupCount, 2)
	}
	if stats.SuccessBackupCount != 1 || stats.FailureBackupCount != 1 {
		t.Fatalf("stats success/failure = (%d, %d), want (1, 1)", stats.SuccessBackupCount, stats.FailureBackupCount)
	}
	if stats.AvailableBackupCount != 1 {
		t.Fatalf("stats.AvailableBackupCount = %d, want %d", stats.AvailableBackupCount, 1)
	}
	if stats.TotalBackupSizeBytes != 60 {
		t.Fatalf("stats.TotalBackupSizeBytes = %d, want %d", stats.TotalBackupSizeBytes, 60)
	}
	if stats.PolicyCount != 1 {
		t.Fatalf("stats.PolicyCount = %d, want %d", stats.PolicyCount, 1)
	}
	if stats.LastBackup == nil || stats.LastBackup.ID != secondBackupID {
		t.Fatalf("stats.LastBackup = %+v, want backup %d", stats.LastBackup, secondBackupID)
	}
	if len(stats.RecentTrend) != 7 {
		t.Fatalf("len(stats.RecentTrend) = %d, want %d", len(stats.RecentTrend), 7)
	}

	lastBackup, err := db.GetLastBackup(instance.ID)
	if err != nil {
		t.Fatalf("GetLastBackup() error = %v", err)
	}
	if lastBackup.ID != secondBackupID {
		t.Fatalf("GetLastBackup().ID = %d, want %d", lastBackup.ID, secondBackupID)
	}

	if err := db.DeleteInstance(instance.ID); err != nil {
		t.Fatalf("DeleteInstance() error = %v", err)
	}
	if _, err := db.GetInstanceByID(instance.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetInstanceByID(deleted) error = %v, want sql.ErrNoRows", err)
	}

	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM policies WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM backups WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM tasks WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM instance_permissions WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM notification_subscriptions WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE instance_id = ?`, instance.ID, 0)
	assertInstanceRelatedRowCount(t, db, `SELECT COUNT(*) FROM instances WHERE id = ?`, second.ID, 1)
}

func TestCountConsecutiveFailuresIgnoresIntermediateRetryFailures(t *testing.T) {
	db := newTestDB(t)
	instance := &model.Instance{
		Name:       "retry-count-instance",
		SourceType: "local",
		SourcePath: "/srv/retry-count-instance",
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "retry-count-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policyID := createStoreTestPolicy(t, db, instance.ID, target.ID, "retry-aware-policy")
	rootSuccess := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('now', '-3 hour')")
	insertStoreTestBackupWithRetryRoot(t, db, instance.ID, policyID, rootSuccess, "success", 64, 32, "datetime('now', '-2 hour')")
	rootFailure := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('now', '-1 hour')")
	insertStoreTestBackupWithRetryRoot(t, db, instance.ID, policyID, rootFailure, "failed", 32, 16, "CURRENT_TIMESTAMP")
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.complete", "datetime('now', '-2 hour')")
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.fail", "CURRENT_TIMESTAMP")

	failures, err := db.CountConsecutiveFailures(instance.ID, policyID)
	if err != nil {
		t.Fatalf("CountConsecutiveFailures() error = %v", err)
	}
	if failures != 1 {
		t.Fatalf("CountConsecutiveFailures() = %d, want %d", failures, 1)
	}
}

func TestListRecentLogicalBackupsByInstanceUsesLatestFinalResults(t *testing.T) {
	db := newTestDB(t)

	instance := &model.Instance{Name: "logical-window", SourceType: "local", SourcePath: "/srv/logical-window", Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:         "logical-window-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  "/backup/logical-window",
		HealthStatus: "healthy",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policyID := createStoreTestPolicy(t, db, instance.ID, target.ID, "logical-window-policy")
	for minute := 0; minute < 10; minute++ {
		status := "success"
		if minute >= 8 {
			status = "failed"
		}
		insertStoreTestBackup(t, db, instance.ID, policyID, status, 32, 16, "datetime('2026-04-07 12:00:00', '-"+strconv.Itoa(minute)+" minutes')")
	}

	oldRetryRoot := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('2026-04-07 12:00:00', '-10 minutes')")
	insertStoreTestBackupWithRetryRoot(t, db, instance.ID, policyID, oldRetryRoot, "success", 64, 32, "datetime('2026-04-07 12:00:00', '-9 minutes', '-30 seconds')")
	insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('2026-04-07 12:00:00', '-11 minutes')")

	backups, err := db.ListRecentLogicalBackupsByInstance(instance.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentLogicalBackupsByInstance() error = %v", err)
	}
	if len(backups) != 10 {
		t.Fatalf("len(backups) = %d, want 10", len(backups))
	}

	successCount := 0
	for _, backup := range backups {
		if backup.Status == "success" {
			successCount++
		}
	}
	if successCount != 8 {
		t.Fatalf("successCount = %d, want 8 from latest ten final results", successCount)
	}
	for _, backup := range backups {
		if backup.ID == oldRetryRoot {
			t.Fatalf("logical backups still include retry root %d instead of final retry result", oldRetryRoot)
		}
	}
}

func TestListRecentBackupExecutionStatusesByInstanceUsesAuditLogs(t *testing.T) {
	db := newTestDB(t)

	instance := &model.Instance{Name: "audit-window", SourceType: "local", SourcePath: "/srv/audit-window", Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:         "audit-window-target",
		BackupType:   "rolling",
		StorageType:  "local",
		StoragePath:  "/backup/audit-window",
		HealthStatus: "healthy",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policyID := createStoreTestPolicy(t, db, instance.ID, target.ID, "audit-window-policy")
	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	retryRoot := insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('2026-04-07 12:00:00', '-1 minute')")
	insertStoreTestBackupWithRetryRoot(t, db, instance.ID, policyID, retryRoot, "success", 64, 32, "datetime('2026-04-07 12:00:00', '-30 seconds')")
	insertStoreTestBackup(t, db, instance.ID, policyID, "failed", 32, 16, "datetime('2026-04-07 12:00:00', '-2 minutes')")
	insertStoreTestBackup(t, db, instance.ID, policyID, "success", 32, 16, "datetime('2026-04-07 12:00:00', '-3 minutes')")

	insertStoreTestBackupAudit(t, db, instance.ID, "backup.complete", "'"+base.Add(-30*time.Second).UTC().Format("2006-01-02 15:04:05")+"'")
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.fail", "'"+base.Add(-2*time.Minute).UTC().Format("2006-01-02 15:04:05")+"'")
	insertStoreTestBackupAudit(t, db, instance.ID, "backup.complete", "'"+base.Add(-3*time.Minute).UTC().Format("2006-01-02 15:04:05")+"'")

	statuses, err := db.ListRecentBackupExecutionStatusesByInstance(instance.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentBackupExecutionStatusesByInstance() error = %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("len(statuses) = %d, want 3 final audit results", len(statuses))
	}
	if statuses[0] != "success" || statuses[1] != "failed" || statuses[2] != "success" {
		t.Fatalf("statuses = %#v, want []string{\"success\", \"failed\", \"success\"}", statuses)
	}
}

func createStoreTestPolicy(t *testing.T, db *DB, instanceID, targetID int64, name string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO policies (instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retention_type, retention_value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		instanceID,
		name,
		"rolling",
		targetID,
		"cron",
		"0 0 * * *",
		true,
		false,
		false,
		nil,
		false,
		nil,
		"count",
		7,
	)
	if err != nil {
		t.Fatalf("insert policy error = %v", err)
	}

	policyID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(policy) error = %v", err)
	}

	return policyID
}

func insertStoreTestBackup(t *testing.T, db *DB, instanceID, policyID int64, status string, backupSize, actualSize int64, completedAtExpr string) int64 {
	t.Helper()
	return insertStoreTestBackupWithNullableRetryRoot(t, db, instanceID, policyID, nil, status, backupSize, actualSize, completedAtExpr)
}

func insertStoreTestBackupWithRetryRoot(t *testing.T, db *DB, instanceID, policyID, retryRootBackupID int64, status string, backupSize, actualSize int64, completedAtExpr string) int64 {
	t.Helper()
	return insertStoreTestBackupWithNullableRetryRoot(t, db, instanceID, policyID, &retryRootBackupID, status, backupSize, actualSize, completedAtExpr)
}

func insertStoreTestBackupWithNullableRetryRoot(t *testing.T, db *DB, instanceID, policyID int64, retryRootBackupID *int64, status string, backupSize, actualSize int64, completedAtExpr string) int64 {
	t.Helper()

	query := `INSERT INTO backups (instance_id, policy_id, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, retry_root_backup_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ` + completedAtExpr + `, ` + completedAtExpr + `, ?, ?, ?, ?, ` + completedAtExpr + `)`
	result, err := db.Exec(
		query,
		instanceID,
		policyID,
		"rolling",
		status,
		"/snapshots/"+status,
		backupSize,
		actualSize,
		60,
		"",
		`{"files":10}`,
		retryRootBackupID,
	)
	if err != nil {
		t.Fatalf("insert backup error = %v", err)
	}

	backupID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(backup) error = %v", err)
	}

	return backupID
}

func insertStoreTestTask(t *testing.T, db *DB, instanceID, backupID int64) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO tasks (instance_id, backup_id, type, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)`,
		instanceID,
		backupID,
		"backup",
		"success",
		100,
		"done",
		"",
	); err != nil {
		t.Fatalf("insert task error = %v", err)
	}
}

func insertStoreTestNotificationSubscription(t *testing.T, db *DB, userID, instanceID int64) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO notification_subscriptions (user_id, instance_id, enabled, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		userID,
		instanceID,
		true,
	); err != nil {
		t.Fatalf("insert notification subscription error = %v", err)
	}
}

func insertStoreTestAuditLog(t *testing.T, db *DB, userID, instanceID int64) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		instanceID,
		userID,
		"instance.deleted",
		"cleanup me",
	); err != nil {
		t.Fatalf("insert audit log error = %v", err)
	}
}

func insertStoreTestBackupAudit(t *testing.T, db *DB, instanceID int64, action string, createdAtExpr string) {
	t.Helper()

	query := `INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at)
		VALUES (?, NULL, ?, ?, ` + createdAtExpr + `)`
	if _, err := db.Exec(query, instanceID, action, `{"source":"test"}`); err != nil {
		t.Fatalf("insert backup audit log error = %v", err)
	}
}

func assertInstanceRelatedRowCount(t *testing.T, db *DB, query string, arg any, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}
	if got != want {
		t.Fatalf("row count for %q = %d, want %d", query, got, want)
	}
}
