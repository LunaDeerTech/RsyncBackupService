package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
)

const (
	policyColumns = `id, instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retry_enabled, retry_max_retries, retention_type, retention_value, created_at, updated_at`
	taskColumns   = `id, instance_id, backup_id, type, restore_type, target_path, remote_config_id, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at`
)

type policyScanner interface {
	Scan(dest ...any) error
}

type taskScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreatePolicy(policy *model.Policy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	result, err := db.Exec(
		`INSERT INTO policies (instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retry_enabled, retry_max_retries, retention_type, retention_value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		policy.InstanceID,
		policy.Name,
		policy.Type,
		policy.TargetID,
		policy.ScheduleType,
		policy.ScheduleValue,
		policy.Enabled,
		policy.Compression,
		policy.Encryption,
		policy.EncryptionKeyHash,
		policy.SplitEnabled,
		policy.SplitSizeMB,
		policy.RetryEnabled,
		policy.RetryMaxRetries,
		policy.RetentionType,
		policy.RetentionValue,
	)
	if err != nil {
		return fmt.Errorf("create policy: %w", err)
	}

	policyID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created policy id: %w", err)
	}

	created, err := db.GetPolicyByID(policyID)
	if err != nil {
		return fmt.Errorf("load created policy: %w", err)
	}

	*policy = *created
	return nil
}

func (db *DB) GetPolicyByID(id int64) (*model.Policy, error) {
	policy, err := scanPolicy(db.QueryRow(`SELECT `+policyColumns+` FROM policies WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get policy by id %d: %w", id, err)
	}

	return policy, nil
}

func (db *DB) ListPoliciesByInstance(instanceID int64) ([]model.Policy, error) {
	rows, err := db.Query(`SELECT `+policyColumns+` FROM policies WHERE instance_id = ? ORDER BY id ASC`, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list policies by instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	policies := make([]model.Policy, 0)
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed policy: %w", err)
		}
		policies = append(policies, *policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policies by instance %d: %w", instanceID, err)
	}

	return policies, nil
}

func (db *DB) UpdatePolicy(policy *model.Policy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	result, err := db.Exec(
		`UPDATE policies
		 SET name = ?, type = ?, target_id = ?, schedule_type = ?, schedule_value = ?, enabled = ?, compression = ?, encryption = ?, encryption_key_hash = ?, split_enabled = ?, split_size_mb = ?, retry_enabled = ?, retry_max_retries = ?, retention_type = ?, retention_value = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		policy.Name,
		policy.Type,
		policy.TargetID,
		policy.ScheduleType,
		policy.ScheduleValue,
		policy.Enabled,
		policy.Compression,
		policy.Encryption,
		policy.EncryptionKeyHash,
		policy.SplitEnabled,
		policy.SplitSizeMB,
		policy.RetryEnabled,
		policy.RetryMaxRetries,
		policy.RetentionType,
		policy.RetentionValue,
		policy.ID,
	)
	if err != nil {
		return fmt.Errorf("update policy %d: %w", policy.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for policy %d: %w", policy.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update policy %d: %w", policy.ID, sql.ErrNoRows)
	}

	updated, err := db.GetPolicyByID(policy.ID)
	if err != nil {
		return fmt.Errorf("load updated policy %d: %w", policy.ID, err)
	}

	*policy = *updated
	return nil
}

func (db *DB) DeletePolicy(id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete policy %d: %w", id, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM tasks WHERE backup_id IN (SELECT id FROM backups WHERE policy_id = ?)`, id); err != nil {
		return fmt.Errorf("delete tasks for policy %d: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM backups WHERE policy_id = ?`, id); err != nil {
		return fmt.Errorf("delete backups for policy %d: %w", id, err)
	}

	result, err := tx.Exec(`DELETE FROM policies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete policy %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for policy %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete policy %d: %w", id, sql.ErrNoRows)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete policy %d: %w", id, err)
	}

	return nil
}

func (db *DB) ListEnabledPolicies() ([]model.Policy, error) {
	rows, err := db.Query(`SELECT ` + policyColumns + ` FROM policies WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list enabled policies: %w", err)
	}
	defer rows.Close()

	policies := make([]model.Policy, 0)
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan enabled policy: %w", err)
		}
		policies = append(policies, *policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled policies: %w", err)
	}

	return policies, nil
}

func (db *DB) ListPolicyExecutionSummaries(instanceID int64) (map[int64]model.PolicyExecutionSummary, error) {
	rows, err := db.Query(
		`SELECT p.id,
		        b.id,
		        b.status,
		        COALESCE(b.completed_at, b.started_at, b.created_at)
		 FROM policies p
		 LEFT JOIN backups b ON b.id = (
		 	SELECT latest.id
		 	FROM backups latest
		 	WHERE latest.policy_id = p.id
		 	ORDER BY COALESCE(latest.completed_at, latest.started_at, latest.created_at) DESC, latest.id DESC
		 	LIMIT 1
		 )
		 WHERE p.instance_id = ?`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list policy execution summaries for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	summaries := make(map[int64]model.PolicyExecutionSummary)
	for rows.Next() {
		var (
			policyID   int64
			backupID   sql.NullInt64
			status     sql.NullString
			rawRunTime sql.NullString
		)

		if err := rows.Scan(&policyID, &backupID, &status, &rawRunTime); err != nil {
			return nil, fmt.Errorf("scan policy execution summary: %w", err)
		}

		summary := model.PolicyExecutionSummary{}
		if backupID.Valid {
			summary.LatestBackupID = &backupID.Int64
		}
		if status.Valid {
			summary.LastExecutionStatus = &status.String
		}
		if rawRunTime.Valid {
			parsed, err := parseSQLiteTime(rawRunTime.String)
			if err != nil {
				return nil, fmt.Errorf("parse policy execution time %q: %w", rawRunTime.String, err)
			}
			summary.LastExecutionTime = &parsed
		}

		summaries[policyID] = summary
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy execution summaries for instance %d: %w", instanceID, err)
	}

	return summaries, nil
}

func (db *DB) CreateBackup(backup *model.Backup) error {
	if backup == nil {
		return fmt.Errorf("backup is nil")
	}

	backup.TriggerSource = normalizeBackupTriggerSource(backup.TriggerSource)

	result, err := db.Exec(
		`INSERT INTO backups (instance_id, policy_id, trigger_source, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		backup.InstanceID,
		backup.PolicyID,
		backup.TriggerSource,
		backup.Type,
		backup.Status,
		backup.SnapshotPath,
		backup.BackupSizeBytes,
		backup.ActualSizeBytes,
		backup.StartedAt,
		backup.CompletedAt,
		backup.DurationSeconds,
		backup.ErrorMessage,
		backup.RsyncStats,
	)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	backupID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created backup id: %w", err)
	}

	created, err := db.GetBackupByID(backupID)
	if err != nil {
		return fmt.Errorf("load created backup: %w", err)
	}

	*backup = *created
	return nil
}

func (db *DB) GetBackupByID(id int64) (*model.Backup, error) {
	backup, err := scanBackup(db.QueryRow(`SELECT `+backupColumns+` FROM backups WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get backup by id %d: %w", id, err)
	}

	return backup, nil
}

func (db *DB) GetLatestSuccessfulBackup(instanceID, policyID int64) (*model.Backup, error) {
	backup, err := scanBackup(db.QueryRow(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE instance_id = ? AND policy_id = ? AND status = 'success'
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT 1`,
		instanceID,
		policyID,
	))
	if err != nil {
		return nil, fmt.Errorf("get latest successful backup for instance %d policy %d: %w", instanceID, policyID, err)
	}

	return backup, nil
}

func (db *DB) GetLatestSuccessfulBackupExcluding(instanceID, policyID, excludeBackupID int64) (*model.Backup, error) {
	backup, err := scanBackup(db.QueryRow(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE instance_id = ? AND policy_id = ? AND status = 'success' AND id != ?
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT 1`,
		instanceID,
		policyID,
		excludeBackupID,
	))
	if err != nil {
		return nil, fmt.Errorf("get latest successful backup for instance %d policy %d excluding %d: %w", instanceID, policyID, excludeBackupID, err)
	}

	return backup, nil
}

func (db *DB) ListExpiredBackups(policyID int64, before time.Time) ([]model.Backup, error) {
	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE policy_id = ?
		   AND status = 'success'
		   AND completed_at IS NOT NULL
		   AND completed_at < ?
		   AND id NOT IN (
			SELECT latest.id
			FROM backups latest
			WHERE latest.policy_id = ? AND latest.status = 'success'
			ORDER BY COALESCE(latest.completed_at, latest.started_at, latest.created_at) DESC, latest.id DESC
			LIMIT 1
		   )
		 ORDER BY completed_at ASC, id ASC`,
		policyID,
		before.UTC(),
		policyID,
	)
	if err != nil {
		return nil, fmt.Errorf("list expired backups for policy %d before %s: %w", policyID, before.UTC().Format(time.RFC3339), err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired backup for policy %d: %w", policyID, err)
		}
		backups = append(backups, *backup)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired backups for policy %d: %w", policyID, err)
	}

	return backups, nil
}

func (db *DB) ListExcessBackups(policyID int64, keepCount int) ([]model.Backup, error) {
	if keepCount < 1 {
		keepCount = 1
	}

	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM (
			SELECT `+backupColumns+`
			FROM backups
			WHERE policy_id = ? AND status = 'success'
			ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
			LIMIT -1 OFFSET ?
		 ) excess
		 ORDER BY COALESCE(completed_at, started_at, created_at) ASC, id ASC`,
		policyID,
		keepCount,
	)
	if err != nil {
		return nil, fmt.Errorf("list excess backups for policy %d keep %d: %w", policyID, keepCount, err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan excess backup for policy %d: %w", policyID, err)
		}
		backups = append(backups, *backup)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate excess backups for policy %d: %w", policyID, err)
	}

	return backups, nil
}

func (db *DB) DeleteBackup(id int64) error {
	result, err := db.Exec(`DELETE FROM backups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete backup %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for backup %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete backup %d: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) DeleteTaskByBackupID(backupID int64) error {
	if _, err := db.Exec(`DELETE FROM tasks WHERE backup_id = ?`, backupID); err != nil {
		return fmt.Errorf("delete tasks for backup %d: %w", backupID, err)
	}

	return nil
}

func (db *DB) GetLatestCompletedBackupByPolicyAndSource(policyID int64, triggerSource string) (*model.Backup, error) {
	backup, err := scanBackup(db.QueryRow(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE policy_id = ? AND trigger_source = ? AND completed_at IS NOT NULL
		 ORDER BY completed_at DESC, id DESC
		 LIMIT 1`,
		policyID,
		normalizeBackupTriggerSource(triggerSource),
	))
	if err != nil {
		return nil, fmt.Errorf("get latest completed backup for policy %d source %q: %w", policyID, triggerSource, err)
	}

	return backup, nil
}

func (db *DB) HasActivePolicyRunBySource(policyID int64, triggerSource string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*)
		 FROM tasks t
		 INNER JOIN backups b ON b.id = t.backup_id
		 WHERE b.policy_id = ? AND b.trigger_source = ? AND t.status IN ('queued', 'running')`,
		policyID,
		normalizeBackupTriggerSource(triggerSource),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check active policy run for policy %d source %q: %w", policyID, triggerSource, err)
	}

	return count > 0, nil
}

func (db *DB) UpdateBackup(backup *model.Backup) error {
	if backup == nil {
		return fmt.Errorf("backup is nil")
	}

	backup.TriggerSource = normalizeBackupTriggerSource(backup.TriggerSource)

	result, err := db.Exec(
		`UPDATE backups
		 SET instance_id = ?, policy_id = ?, trigger_source = ?, type = ?, status = ?, snapshot_path = ?, backup_size_bytes = ?, actual_size_bytes = ?, started_at = ?, completed_at = ?, duration_seconds = ?, error_message = ?, rsync_stats = ?
		 WHERE id = ?`,
		backup.InstanceID,
		backup.PolicyID,
		backup.TriggerSource,
		backup.Type,
		backup.Status,
		backup.SnapshotPath,
		backup.BackupSizeBytes,
		backup.ActualSizeBytes,
		backup.StartedAt,
		backup.CompletedAt,
		backup.DurationSeconds,
		backup.ErrorMessage,
		backup.RsyncStats,
		backup.ID,
	)
	if err != nil {
		return fmt.Errorf("update backup %d: %w", backup.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for backup %d: %w", backup.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update backup %d: %w", backup.ID, sql.ErrNoRows)
	}

	updated, err := db.GetBackupByID(backup.ID)
	if err != nil {
		return fmt.Errorf("load updated backup %d: %w", backup.ID, err)
	}

	*backup = *updated
	return nil
}

func (db *DB) CreateTask(task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	result, err := db.Exec(
		`INSERT INTO tasks (instance_id, backup_id, type, restore_type, target_path, remote_config_id, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		task.InstanceID,
		task.BackupID,
		task.Type,
		task.RestoreType,
		task.TargetPath,
		task.RemoteConfigID,
		task.Status,
		task.Progress,
		task.CurrentStep,
		task.StartedAt,
		task.CompletedAt,
		task.EstimatedEnd,
		task.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	taskID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created task id: %w", err)
	}

	created, err := db.GetTaskByID(taskID)
	if err != nil {
		return fmt.Errorf("load created task: %w", err)
	}

	*task = *created
	return nil
}

func (db *DB) GetTaskByID(id int64) (*model.Task, error) {
	task, err := scanTask(db.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get task by id %d: %w", id, err)
	}

	return task, nil
}

func (db *DB) UpdateTask(task *model.Task) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	result, err := db.Exec(
		`UPDATE tasks
		 SET instance_id = ?, backup_id = ?, type = ?, restore_type = ?, target_path = ?, status = ?, progress = ?, current_step = ?, started_at = ?, completed_at = ?, estimated_end = ?, error_message = ?
		 WHERE id = ?`,
		task.InstanceID,
		task.BackupID,
		task.Type,
		task.RestoreType,
		task.TargetPath,
		task.Status,
		task.Progress,
		task.CurrentStep,
		task.StartedAt,
		task.CompletedAt,
		task.EstimatedEnd,
		task.ErrorMessage,
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %d: %w", task.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for task %d: %w", task.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update task %d: %w", task.ID, sql.ErrNoRows)
	}

	updated, err := db.GetTaskByID(task.ID)
	if err != nil {
		return fmt.Errorf("load updated task %d: %w", task.ID, err)
	}

	*task = *updated
	return nil
}

func (db *DB) ListActiveTasks() ([]model.Task, error) {
	rows, err := db.Query(
		`SELECT `+taskColumns+`
		 FROM tasks
		 WHERE status IN ('queued', 'running')
		 ORDER BY CASE status WHEN 'running' THEN 0 ELSE 1 END, created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active task: %w", err)
		}
		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active tasks: %w", err)
	}

	return tasks, nil
}

func (db *DB) ListTasksByInstance(instanceID int64) ([]model.Task, error) {
	rows, err := db.Query(
		`SELECT `+taskColumns+`
		 FROM tasks
		 WHERE instance_id = ?
		 ORDER BY created_at DESC, id DESC`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks by instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task for instance %d: %w", instanceID, err)
		}
		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks by instance %d: %w", instanceID, err)
	}

	return tasks, nil
}

func (db *DB) HasRunningTask(instanceID int64) (bool, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE instance_id = ? AND status = 'running'`, instanceID).Scan(&count); err != nil {
		return false, fmt.Errorf("check running task for instance %d: %w", instanceID, err)
	}

	return count > 0, nil
}

func (db *DB) GetQueuedTasksByInstance(instanceID int64) ([]model.Task, error) {
	rows, err := db.Query(
		`SELECT `+taskColumns+`
		 FROM tasks
		 WHERE instance_id = ? AND status = 'queued'
		 ORDER BY created_at ASC, id ASC`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list queued tasks by instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan queued task for instance %d: %w", instanceID, err)
		}
		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queued tasks by instance %d: %w", instanceID, err)
	}

	return tasks, nil
}

func (db *DB) CreatePendingPolicyRun(policy *model.Policy) (*model.Backup, *model.Task, error) {
	return db.CreatePendingPolicyRunWithSource(policy, model.BackupTriggerSourceManual)
}

func (db *DB) CreatePendingPolicyRunWithSource(policy *model.Policy, triggerSource string) (*model.Backup, *model.Task, error) {
	if policy == nil {
		return nil, nil, fmt.Errorf("policy is nil")
	}

	triggerSource = normalizeBackupTriggerSource(triggerSource)

	tx, err := db.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("begin create pending policy run for policy %d: %w", policy.ID, err)
	}
	defer tx.Rollback()

	backupResult, err := tx.Exec(
		`INSERT INTO backups (instance_id, policy_id, trigger_source, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		policy.InstanceID,
		policy.ID,
		triggerSource,
		policy.Type,
		"pending",
		"",
		0,
		0,
		nil,
		nil,
		0,
		"",
		"",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create pending backup for policy %d: %w", policy.ID, err)
	}

	backupID, err := backupResult.LastInsertId()
	if err != nil {
		return nil, nil, fmt.Errorf("read created pending backup id for policy %d: %w", policy.ID, err)
	}

	backup, err := scanBackup(tx.QueryRow(`SELECT `+backupColumns+` FROM backups WHERE id = ?`, backupID))
	if err != nil {
		return nil, nil, fmt.Errorf("load created pending backup %d: %w", backupID, err)
	}

	taskResult, err := tx.Exec(
		`INSERT INTO tasks (instance_id, backup_id, type, restore_type, target_path, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		policy.InstanceID,
		backupID,
		policy.Type,
		"",
		"",
		"queued",
		0,
		"queued",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create pending task for policy %d: %w", policy.ID, err)
	}

	taskID, err := taskResult.LastInsertId()
	if err != nil {
		return nil, nil, fmt.Errorf("read created pending task id for policy %d: %w", policy.ID, err)
	}

	task, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, taskID))
	if err != nil {
		return nil, nil, fmt.Errorf("load created pending task %d: %w", taskID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit create pending policy run for policy %d: %w", policy.ID, err)
	}

	return backup, task, nil
}

func normalizeBackupTriggerSource(triggerSource string) string {
	switch strings.ToLower(strings.TrimSpace(triggerSource)) {
	case model.BackupTriggerSourceScheduled:
		return model.BackupTriggerSourceScheduled
	default:
		return model.BackupTriggerSourceManual
	}
}

func scanPolicy(scanner policyScanner) (*model.Policy, error) {
	var (
		policy            model.Policy
		encryptionKeyHash sql.NullString
		splitSizeMB       sql.NullInt64
		rawCreated        string
		rawUpdated        string
	)

	if err := scanner.Scan(
		&policy.ID,
		&policy.InstanceID,
		&policy.Name,
		&policy.Type,
		&policy.TargetID,
		&policy.ScheduleType,
		&policy.ScheduleValue,
		&policy.Enabled,
		&policy.Compression,
		&policy.Encryption,
		&encryptionKeyHash,
		&policy.SplitEnabled,
		&splitSizeMB,
		&policy.RetryEnabled,
		&policy.RetryMaxRetries,
		&policy.RetentionType,
		&policy.RetentionValue,
		&rawCreated,
		&rawUpdated,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreated, err)
	}
	updatedAt, err := parseSQLiteTime(rawUpdated)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", rawUpdated, err)
	}

	policy.CreatedAt = createdAt
	policy.UpdatedAt = updatedAt
	if encryptionKeyHash.Valid {
		policy.EncryptionKeyHash = &encryptionKeyHash.String
	}
	if splitSizeMB.Valid {
		value := int(splitSizeMB.Int64)
		policy.SplitSizeMB = &value
	}

	return &policy, nil
}

func scanTask(scanner taskScanner) (*model.Task, error) {
	var (
		task           model.Task
		backupID       sql.NullInt64
		restoreType    string
		targetPath     string
		remoteConfigID sql.NullInt64
		startedAt      sql.NullString
		completedAt    sql.NullString
		estimatedEnd   sql.NullString
		rawCreatedAt   string
	)

	if err := scanner.Scan(
		&task.ID,
		&task.InstanceID,
		&backupID,
		&task.Type,
		&restoreType,
		&targetPath,
		&remoteConfigID,
		&task.Status,
		&task.Progress,
		&task.CurrentStep,
		&startedAt,
		&completedAt,
		&estimatedEnd,
		&task.ErrorMessage,
		&rawCreatedAt,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreatedAt, err)
	}
	if backupID.Valid {
		task.BackupID = &backupID.Int64
	}
	if remoteConfigID.Valid {
		task.RemoteConfigID = &remoteConfigID.Int64
	}
	task.RestoreType = restoreType
	task.TargetPath = targetPath
	if startedAt.Valid {
		parsed, err := parseSQLiteTime(startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at %q: %w", startedAt.String, err)
		}
		task.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed, err := parseSQLiteTime(completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at %q: %w", completedAt.String, err)
		}
		task.CompletedAt = &parsed
	}
	if estimatedEnd.Valid {
		parsed, err := parseSQLiteTime(estimatedEnd.String)
		if err != nil {
			return nil, fmt.Errorf("parse estimated_end %q: %w", estimatedEnd.String, err)
		}
		task.EstimatedEnd = &parsed
	}
	task.CreatedAt = createdAt

	return &task, nil
}
