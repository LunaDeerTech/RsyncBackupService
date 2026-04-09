package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
)

const (
	instanceColumns = `id, name, source_type, source_path, exclude_patterns, remote_config_id, status, created_at, updated_at`
	backupColumns   = `id, instance_id, policy_id, trigger_source, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, created_at`
)

type instanceScanner interface {
	Scan(dest ...any) error
}

type backupScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreateInstance(inst *model.Instance) error {
	if inst == nil {
		return fmt.Errorf("instance is nil")
	}

	result, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, exclude_patterns, remote_config_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		inst.Name,
		inst.SourceType,
		inst.SourcePath,
		joinExcludePatterns(inst.ExcludePatterns),
		inst.RemoteConfigID,
		inst.Status,
	)
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}

	instanceID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created instance id: %w", err)
	}

	created, err := db.GetInstanceByID(instanceID)
	if err != nil {
		return fmt.Errorf("load created instance: %w", err)
	}

	*inst = *created
	return nil
}

func (db *DB) GetInstanceByID(id int64) (*model.Instance, error) {
	instance, err := scanInstance(db.QueryRow(`SELECT `+instanceColumns+` FROM instances WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get instance by id %d: %w", id, err)
	}

	return instance, nil
}

func (db *DB) GetInstanceByName(name string) (*model.Instance, error) {
	instance, err := scanInstance(db.QueryRow(`SELECT `+instanceColumns+` FROM instances WHERE name = ?`, strings.TrimSpace(name)))
	if err != nil {
		return nil, fmt.Errorf("get instance by name %q: %w", name, err)
	}

	return instance, nil
}

func (db *DB) ListInstances() ([]model.Instance, error) {
	rows, err := db.Query(`SELECT ` + instanceColumns + ` FROM instances ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	instances := make([]model.Instance, 0)
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed instance: %w", err)
		}
		instances = append(instances, *instance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances: %w", err)
	}

	return instances, nil
}

func (db *DB) ListInstancesPage(limit, offset int) ([]model.Instance, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list instances page: limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list instances page: offset must be non-negative")
	}

	rows, err := db.Query(`SELECT `+instanceColumns+` FROM instances ORDER BY id ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list instances page limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	instances := make([]model.Instance, 0, limit)
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed instance page: %w", err)
		}
		instances = append(instances, *instance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance page: %w", err)
	}

	return instances, nil
}

func (db *DB) CountInstances() (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM instances`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count instances: %w", err)
	}

	return count, nil
}

func (db *DB) ListInstancesByUserPermission(userID int64) ([]model.Instance, error) {
	rows, err := db.Query(
		`SELECT `+instanceColumns+`
		 FROM instances
		 WHERE id IN (
			SELECT instance_id FROM instance_permissions WHERE user_id = ?
		 )
		 ORDER BY id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list instances by user permission %d: %w", userID, err)
	}
	defer rows.Close()

	instances := make([]model.Instance, 0)
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed instance by user permission %d: %w", userID, err)
		}
		instances = append(instances, *instance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances by user permission %d: %w", userID, err)
	}

	return instances, nil
}

func (db *DB) ListInstancesByUserPermissionPage(userID int64, limit, offset int) ([]model.Instance, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list instances by user permission page: limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list instances by user permission page: offset must be non-negative")
	}

	rows, err := db.Query(
		`SELECT `+instanceColumns+`
		 FROM instances
		 WHERE id IN (
			SELECT instance_id FROM instance_permissions WHERE user_id = ?
		 )
		 ORDER BY id ASC
		 LIMIT ? OFFSET ?`,
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list instances by user permission %d page limit %d offset %d: %w", userID, limit, offset, err)
	}
	defer rows.Close()

	instances := make([]model.Instance, 0, limit)
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed instance by user permission page %d: %w", userID, err)
		}
		instances = append(instances, *instance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances by user permission page %d: %w", userID, err)
	}

	return instances, nil
}

func (db *DB) CountInstancesByUserPermission(userID int64) (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM instance_permissions WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count instances by user permission %d: %w", userID, err)
	}

	return count, nil
}

func (db *DB) UpdateInstance(inst *model.Instance) error {
	if inst == nil {
		return fmt.Errorf("instance is nil")
	}

	result, err := db.Exec(
		`UPDATE instances
		 SET name = ?, source_type = ?, source_path = ?, exclude_patterns = ?, remote_config_id = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		inst.Name,
		inst.SourceType,
		inst.SourcePath,
		joinExcludePatterns(inst.ExcludePatterns),
		inst.RemoteConfigID,
		inst.Status,
		inst.ID,
	)
	if err != nil {
		return fmt.Errorf("update instance %d: %w", inst.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for instance %d: %w", inst.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update instance %d: %w", inst.ID, sql.ErrNoRows)
	}

	updated, err := db.GetInstanceByID(inst.ID)
	if err != nil {
		return fmt.Errorf("load updated instance %d: %w", inst.ID, err)
	}

	*inst = *updated
	return nil
}

func (db *DB) DeleteInstance(id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete instance %d: %w", id, err)
	}
	defer tx.Rollback()

	statements := []string{
		`DELETE FROM notification_subscriptions WHERE instance_id = ?`,
		`DELETE FROM instance_permissions WHERE instance_id = ?`,
		`DELETE FROM tasks WHERE instance_id = ?`,
		`DELETE FROM backups WHERE instance_id = ?`,
		`DELETE FROM policies WHERE instance_id = ?`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement, id); err != nil {
			return fmt.Errorf("delete related rows for instance %d: %w", id, err)
		}
	}

	result, err := tx.Exec(`DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete instance %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for instance %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete instance %d: %w", id, sql.ErrNoRows)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete instance %d: %w", id, err)
	}

	return nil
}

func (db *DB) UpdateInstanceStatus(id int64, status string) error {
	result, err := db.Exec(`UPDATE instances SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, strings.TrimSpace(status), id)
	if err != nil {
		return fmt.Errorf("update instance %d status: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for instance %d status: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("update instance %d status: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) ResetAllRunningInstances() error {
	_, err := db.Exec(`UPDATE instances SET status = 'idle', updated_at = CURRENT_TIMESTAMP WHERE status = 'running'`)
	if err != nil {
		return fmt.Errorf("reset running instances: %w", err)
	}
	return nil
}

func (db *DB) GetInstanceStats(instanceID int64) (*model.InstanceStats, error) {
	stats := &model.InstanceStats{}

	if err := db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(actual_size_bytes), 0)
		 FROM backups
		 WHERE instance_id = ?`,
		instanceID,
	).Scan(&stats.BackupCount, &stats.SuccessBackupCount, &stats.FailureBackupCount, &stats.TotalBackupSizeBytes); err != nil {
		return nil, fmt.Errorf("get instance %d backup stats: %w", instanceID, err)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM policies WHERE instance_id = ?`, instanceID).Scan(&stats.PolicyCount); err != nil {
		return nil, fmt.Errorf("get instance %d policy count: %w", instanceID, err)
	}

	lastBackup, err := db.GetLastBackup(instanceID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		lastBackup = nil
	}
	stats.LastBackup = lastBackup

	rows, err := db.Query(
		`WITH RECURSIVE days(day) AS (
			SELECT date('now', '-6 days')
			UNION ALL
			SELECT date(day, '+1 day') FROM days WHERE day < date('now')
		 )
		 SELECT days.day,
		        COUNT(backups.id),
		        COALESCE(SUM(CASE WHEN backups.status = 'success' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN backups.status = 'failed' THEN 1 ELSE 0 END), 0)
		 FROM days
		 LEFT JOIN backups
		   ON backups.instance_id = ?
		  AND substr(COALESCE(backups.completed_at, backups.started_at, backups.created_at), 1, 10) = days.day
		 GROUP BY days.day
		 ORDER BY days.day ASC`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("get instance %d trend: %w", instanceID, err)
	}
	defer rows.Close()

	stats.RecentTrend = make([]model.BackupTrendPoint, 0, 7)
	for rows.Next() {
		var point model.BackupTrendPoint
		if err := rows.Scan(&point.Date, &point.Count, &point.SuccessCount, &point.FailureCount); err != nil {
			return nil, fmt.Errorf("scan instance %d trend: %w", instanceID, err)
		}
		stats.RecentTrend = append(stats.RecentTrend, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance %d trend: %w", instanceID, err)
	}

	return stats, nil
}

func (db *DB) GetLastBackup(instanceID int64) (*model.Backup, error) {
	backup, err := scanBackup(db.QueryRow(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE instance_id = ?
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT 1`,
		instanceID,
	))
	if err != nil {
		return nil, fmt.Errorf("get last backup for instance %d: %w", instanceID, err)
	}

	return backup, nil
}

func (db *DB) ListBackupsByInstance(instanceID int64, limit, offset int) ([]model.Backup, error) {
	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE instance_id = ? AND status = 'success'
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT ? OFFSET ?`,
		instanceID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list backups for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		b, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan backup row: %w", err)
		}
		backups = append(backups, *b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backup rows: %w", err)
	}
	return backups, nil
}

func (db *DB) CountBackupsByInstance(instanceID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM backups WHERE instance_id = ? AND status = 'success'`, instanceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count backups for instance %d: %w", instanceID, err)
	}
	return count, nil
}

func scanInstance(scanner instanceScanner) (*model.Instance, error) {
	var (
		instance           model.Instance
		rawExcludePatterns string
		remoteConfigID     sql.NullInt64
		rawCreated         string
		rawUpdated         string
	)

	if err := scanner.Scan(
		&instance.ID,
		&instance.Name,
		&instance.SourceType,
		&instance.SourcePath,
		&rawExcludePatterns,
		&remoteConfigID,
		&instance.Status,
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

	instance.CreatedAt = createdAt
	instance.UpdatedAt = updatedAt
	instance.ExcludePatterns = splitExcludePatterns(rawExcludePatterns)
	if remoteConfigID.Valid {
		instance.RemoteConfigID = &remoteConfigID.Int64
	}

	return &instance, nil
}

func joinExcludePatterns(patterns []string) string {
	normalized := model.NormalizeExcludePatterns(patterns)
	if len(normalized) == 0 {
		return ""
	}

	return strings.Join(normalized, "\n")
}

func splitExcludePatterns(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return model.NormalizeExcludePatterns(strings.Split(trimmed, "\n"))
}

func scanBackup(scanner backupScanner) (*model.Backup, error) {
	var (
		backup       model.Backup
		startedAt    sql.NullString
		completedAt  sql.NullString
		rawCreatedAt string
	)

	if err := scanner.Scan(
		&backup.ID,
		&backup.InstanceID,
		&backup.PolicyID,
		&backup.TriggerSource,
		&backup.Type,
		&backup.Status,
		&backup.SnapshotPath,
		&backup.BackupSizeBytes,
		&backup.ActualSizeBytes,
		&startedAt,
		&completedAt,
		&backup.DurationSeconds,
		&backup.ErrorMessage,
		&backup.RsyncStats,
		&rawCreatedAt,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreatedAt, err)
	}
	backup.CreatedAt = createdAt

	if startedAt.Valid {
		parsedStartedAt, err := parseSQLiteTime(startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at %q: %w", startedAt.String, err)
		}
		backup.StartedAt = &parsedStartedAt
	}
	if completedAt.Valid {
		parsedCompletedAt, err := parseSQLiteTime(completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at %q: %w", completedAt.String, err)
		}
		backup.CompletedAt = &parsedCompletedAt
	}

	return &backup, nil
}
