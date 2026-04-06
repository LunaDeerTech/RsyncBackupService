package store

import (
	"database/sql"
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
)

const backupTargetColumns = `id, name, backup_type, storage_type, storage_path, remote_config_id, total_capacity_bytes, used_capacity_bytes, last_health_check, health_status, health_message, created_at, updated_at`

type BackupTargetUsage struct {
	Policies []string `json:"policies,omitempty"`
}

type backupTargetScanner interface {
	Scan(dest ...any) error
}

func (u BackupTargetUsage) InUse() bool {
	return len(u.Policies) > 0
}

func (db *DB) CreateBackupTarget(target *model.BackupTarget) error {
	if target == nil {
		return fmt.Errorf("backup target is nil")
	}

	result, err := db.Exec(
		`INSERT INTO backup_targets (name, backup_type, storage_type, storage_path, remote_config_id, total_capacity_bytes, used_capacity_bytes, last_health_check, health_status, health_message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		target.Name,
		target.BackupType,
		target.StorageType,
		target.StoragePath,
		target.RemoteConfigID,
		target.TotalCapacityBytes,
		target.UsedCapacityBytes,
		target.LastHealthCheck,
		target.HealthStatus,
		target.HealthMessage,
	)
	if err != nil {
		return fmt.Errorf("create backup target: %w", err)
	}

	targetID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created backup target id: %w", err)
	}

	created, err := db.GetBackupTargetByID(targetID)
	if err != nil {
		return fmt.Errorf("load created backup target: %w", err)
	}

	*target = *created
	return nil
}

func (db *DB) GetBackupTargetByID(id int64) (*model.BackupTarget, error) {
	target, err := scanBackupTarget(db.QueryRow(`SELECT `+backupTargetColumns+` FROM backup_targets WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get backup target by id %d: %w", id, err)
	}

	return target, nil
}

func (db *DB) GetBackupTargetByName(name string) (*model.BackupTarget, error) {
	target, err := scanBackupTarget(db.QueryRow(`SELECT `+backupTargetColumns+` FROM backup_targets WHERE name = ?`, strings.TrimSpace(name)))
	if err != nil {
		return nil, fmt.Errorf("get backup target by name %q: %w", name, err)
	}

	return target, nil
}

func (db *DB) ListBackupTargets() ([]model.BackupTarget, error) {
	rows, err := db.Query(`SELECT ` + backupTargetColumns + ` FROM backup_targets ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list backup targets: %w", err)
	}
	defer rows.Close()

	targets := make([]model.BackupTarget, 0)
	for rows.Next() {
		target, err := scanBackupTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed backup target: %w", err)
		}
		targets = append(targets, *target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backup targets: %w", err)
	}

	return targets, nil
}

func (db *DB) ListBackupTargetsPage(limit, offset int) ([]model.BackupTarget, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list backup targets page: limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list backup targets page: offset must be non-negative")
	}

	rows, err := db.Query(
		`SELECT `+backupTargetColumns+` FROM backup_targets ORDER BY id ASC LIMIT ? OFFSET ?`,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list backup targets page limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	targets := make([]model.BackupTarget, 0, limit)
	for rows.Next() {
		target, err := scanBackupTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed backup target page: %w", err)
		}
		targets = append(targets, *target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backup target page: %w", err)
	}

	return targets, nil
}

func (db *DB) CountBackupTargets() (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM backup_targets`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count backup targets: %w", err)
	}

	return count, nil
}

func (db *DB) UpdateBackupTarget(target *model.BackupTarget) error {
	if target == nil {
		return fmt.Errorf("backup target is nil")
	}

	result, err := db.Exec(
		`UPDATE backup_targets
		 SET name = ?, backup_type = ?, storage_type = ?, storage_path = ?, remote_config_id = ?, total_capacity_bytes = ?, used_capacity_bytes = ?, last_health_check = ?, health_status = ?, health_message = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		target.Name,
		target.BackupType,
		target.StorageType,
		target.StoragePath,
		target.RemoteConfigID,
		target.TotalCapacityBytes,
		target.UsedCapacityBytes,
		target.LastHealthCheck,
		target.HealthStatus,
		target.HealthMessage,
		target.ID,
	)
	if err != nil {
		return fmt.Errorf("update backup target %d: %w", target.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for backup target %d: %w", target.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update backup target %d: %w", target.ID, sql.ErrNoRows)
	}

	updated, err := db.GetBackupTargetByID(target.ID)
	if err != nil {
		return fmt.Errorf("load updated backup target %d: %w", target.ID, err)
	}

	*target = *updated
	return nil
}

func (db *DB) DeleteBackupTarget(id int64) error {
	result, err := db.Exec(`DELETE FROM backup_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete backup target %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for backup target %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete backup target %d: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) IsBackupTargetInUse(id int64) (bool, error) {
	usage, err := db.GetBackupTargetUsage(id)
	if err != nil {
		return false, err
	}

	return usage.InUse(), nil
}

func (db *DB) GetBackupTargetUsage(id int64) (BackupTargetUsage, error) {
	policies, err := db.listReferenceNames(`SELECT name FROM policies WHERE target_id = ? ORDER BY name ASC`, id)
	if err != nil {
		return BackupTargetUsage{}, fmt.Errorf("list policies using backup target %d: %w", id, err)
	}

	return BackupTargetUsage{Policies: policies}, nil
}

func (db *DB) UpdateHealthStatus(id int64, status, message string, total, used *int64) error {
	result, err := db.Exec(
		`UPDATE backup_targets
		 SET health_status = ?, health_message = ?, total_capacity_bytes = ?, used_capacity_bytes = ?, last_health_check = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		status,
		message,
		total,
		used,
		id,
	)
	if err != nil {
		return fmt.Errorf("update backup target %d health status: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for backup target %d health status: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("update backup target %d health status: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) listReferenceNames(query string, id int64) ([]string, error) {
	rows, err := db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return names, nil
}

func scanBackupTarget(scanner backupTargetScanner) (*model.BackupTarget, error) {
	var (
		target          model.BackupTarget
		remoteConfigID  sql.NullInt64
		totalCapacity   sql.NullInt64
		usedCapacity    sql.NullInt64
		lastHealthCheck sql.NullString
		rawCreated      string
		rawUpdated      string
	)

	if err := scanner.Scan(
		&target.ID,
		&target.Name,
		&target.BackupType,
		&target.StorageType,
		&target.StoragePath,
		&remoteConfigID,
		&totalCapacity,
		&usedCapacity,
		&lastHealthCheck,
		&target.HealthStatus,
		&target.HealthMessage,
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

	target.CreatedAt = createdAt
	target.UpdatedAt = updatedAt
	if remoteConfigID.Valid {
		target.RemoteConfigID = &remoteConfigID.Int64
	}
	if totalCapacity.Valid {
		target.TotalCapacityBytes = &totalCapacity.Int64
	}
	if usedCapacity.Valid {
		target.UsedCapacityBytes = &usedCapacity.Int64
	}
	if lastHealthCheck.Valid {
		parsed, err := parseSQLiteTime(lastHealthCheck.String)
		if err != nil {
			return nil, fmt.Errorf("parse last_health_check %q: %w", lastHealthCheck.String, err)
		}
		target.LastHealthCheck = &parsed
	}

	return &target, nil
}
