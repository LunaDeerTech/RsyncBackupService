package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rsync-backup-service/internal/model"
)

func (db *DB) GetLatestSuccessfulBackupByPolicies(instanceID int64, policyIDs []int64) (*model.Backup, error) {
	if len(policyIDs) == 0 {
		return nil, sql.ErrNoRows
	}

	var latest *model.Backup
	for _, policyID := range policyIDs {
		backup, err := db.GetLatestSuccessfulBackup(instanceID, policyID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		if latest == nil || backupCompletedAt(backup).After(backupCompletedAt(latest)) {
			latest = backup
		}
	}

	if latest == nil {
		return nil, sql.ErrNoRows
	}

	return latest, nil
}

func (db *DB) ListRecentBackupsByInstanceAllStatuses(instanceID int64, limit int) ([]model.Backup, error) {
	if limit <= 0 {
		return []model.Backup{}, nil
	}

	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE instance_id = ?
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT ?`,
		instanceID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent backups for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0, limit)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recent backup for instance %d: %w", instanceID, err)
		}
		backups = append(backups, *backup)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent backups for instance %d: %w", instanceID, err)
	}

	return backups, nil
}

func (db *DB) ListRecentBackupExecutionStatusesByInstance(instanceID int64, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	rows, err := db.Query(
		`SELECT action
		 FROM audit_logs
		 WHERE instance_id = ? AND action IN (?, ?)
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		instanceID,
		backupCompleteAction,
		backupFailAction,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent backup execution statuses for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	statuses := make([]string, 0, limit)
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return nil, fmt.Errorf("scan recent backup execution status for instance %d: %w", instanceID, err)
		}

		switch action {
		case backupCompleteAction:
			statuses = append(statuses, "success")
		case backupFailAction:
			statuses = append(statuses, "failed")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent backup execution statuses for instance %d: %w", instanceID, err)
	}

	return statuses, nil
}

func (db *DB) ListRecentLogicalBackupsByInstance(instanceID int64, limit int) ([]model.Backup, error) {
	if limit <= 0 {
		return []model.Backup{}, nil
	}

	rows, err := db.Query(
		`WITH ranked_backups AS (
			SELECT `+backupColumns+`,
			       ROW_NUMBER() OVER (
				   PARTITION BY COALESCE(retry_root_backup_id, id)
				   ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
			   ) AS rn
			FROM backups
			WHERE instance_id = ?
		)
		SELECT `+backupColumns+`
		FROM ranked_backups
		WHERE rn = 1 AND status IN ('success', 'failed')
		ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		LIMIT ?`,
		instanceID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent logical backups for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0, limit)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recent logical backup for instance %d: %w", instanceID, err)
		}
		backups = append(backups, *backup)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent logical backups for instance %d: %w", instanceID, err)
	}

	return backups, nil
}

func (db *DB) ListSuccessfulBackupsByPolicy(policyID int64, limit int) ([]model.Backup, error) {
	if limit <= 0 {
		return []model.Backup{}, nil
	}

	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE policy_id = ? AND status = 'success'
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC
		 LIMIT ?`,
		policyID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list successful backups for policy %d: %w", policyID, err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0, limit)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan successful backup for policy %d: %w", policyID, err)
		}
		backups = append(backups, *backup)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate successful backups for policy %d: %w", policyID, err)
	}

	return backups, nil
}

func (db *DB) ListSuccessfulBackupsByPolicySince(policyID int64, since time.Time) ([]model.Backup, error) {
	rows, err := db.Query(
		`SELECT `+backupColumns+`
		 FROM backups
		 WHERE policy_id = ?
		   AND status = 'success'
		   AND COALESCE(completed_at, started_at, created_at) >= ?
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC`,
		policyID,
		since.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("list successful backups since for policy %d: %w", policyID, err)
	}
	defer rows.Close()

	backups := make([]model.Backup, 0)
	for rows.Next() {
		backup, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan successful backup since for policy %d: %w", policyID, err)
		}
		backups = append(backups, *backup)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate successful backups since for policy %d: %w", policyID, err)
	}

	return backups, nil
}

func (db *DB) ListInstanceIDsByTargetID(targetID int64) ([]int64, error) {
	rows, err := db.Query(
		`SELECT DISTINCT instance_id
		 FROM policies
		 WHERE target_id = ?
		 ORDER BY instance_id ASC`,
		targetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list instance ids by target %d: %w", targetID, err)
	}
	defer rows.Close()

	instanceIDs := make([]int64, 0)
	for rows.Next() {
		var instanceID int64
		if err := rows.Scan(&instanceID); err != nil {
			return nil, fmt.Errorf("scan instance id by target %d: %w", targetID, err)
		}
		instanceIDs = append(instanceIDs, instanceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance ids by target %d: %w", targetID, err)
	}

	return instanceIDs, nil
}

func (db *DB) ListUnresolvedRiskEventMessagesByInstance(instanceID int64) ([]string, error) {
	rows, err := db.Query(
		`SELECT message
		 FROM risk_events
		 WHERE instance_id = ? AND resolved = 0
		 ORDER BY created_at DESC, id DESC`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list unresolved risk events for instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	messages := make([]string, 0)
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return nil, fmt.Errorf("scan unresolved risk event for instance %d: %w", instanceID, err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unresolved risk events for instance %d: %w", instanceID, err)
	}

	return messages, nil
}

func backupCompletedAt(backup *model.Backup) time.Time {
	if backup == nil {
		return time.Time{}
	}
	if backup.CompletedAt != nil {
		return backup.CompletedAt.UTC()
	}
	if backup.StartedAt != nil {
		return backup.StartedAt.UTC()
	}
	return backup.CreatedAt.UTC()
}
