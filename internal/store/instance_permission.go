package store

import (
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
)

const instancePermissionColumns = `id, user_id, instance_id, permission, created_at`

type instancePermissionScanner interface {
	Scan(dest ...any) error
}

func (db *DB) GetInstancePermission(userID, instanceID int64) (*model.InstancePermission, error) {
	permission, err := scanInstancePermission(db.QueryRow(
		`SELECT `+instancePermissionColumns+` FROM instance_permissions WHERE user_id = ? AND instance_id = ?`,
		userID,
		instanceID,
	))
	if err != nil {
		return nil, fmt.Errorf("get instance permission user %d instance %d: %w", userID, instanceID, err)
	}

	return permission, nil
}

func (db *DB) SetInstancePermissions(instanceID int64, permissions []model.InstancePermission) error {
	if instanceID <= 0 {
		return fmt.Errorf("instance id is required")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin set instance permissions for instance %d: %w", instanceID, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM instance_permissions WHERE instance_id = ?`, instanceID); err != nil {
		return fmt.Errorf("clear instance permissions for instance %d: %w", instanceID, err)
	}

	for _, permission := range permissions {
		if permission.UserID <= 0 {
			return fmt.Errorf("instance permission user id is required")
		}
		name := strings.TrimSpace(permission.Permission)
		if name == "" {
			return fmt.Errorf("instance permission value is required")
		}

		if _, err := tx.Exec(
			`INSERT INTO instance_permissions (user_id, instance_id, permission, created_at)
			 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			permission.UserID,
			instanceID,
			name,
		); err != nil {
			return fmt.Errorf("insert instance permission user %d instance %d: %w", permission.UserID, instanceID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit instance permissions for instance %d: %w", instanceID, err)
	}

	return nil
}

func (db *DB) ListInstancePermissionsByUser(userID int64) ([]model.InstancePermission, error) {
	rows, err := db.Query(
		`SELECT `+instancePermissionColumns+` FROM instance_permissions WHERE user_id = ? ORDER BY instance_id ASC, id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list instance permissions for user %d: %w", userID, err)
	}
	defer rows.Close()

	permissions := make([]model.InstancePermission, 0)
	for rows.Next() {
		permission, err := scanInstancePermission(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed instance permission for user %d: %w", userID, err)
		}
		permissions = append(permissions, *permission)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance permissions for user %d: %w", userID, err)
	}

	return permissions, nil
}

func scanInstancePermission(scanner instancePermissionScanner) (*model.InstancePermission, error) {
	var (
		permission model.InstancePermission
		rawCreated string
	)

	if err := scanner.Scan(
		&permission.ID,
		&permission.UserID,
		&permission.InstanceID,
		&permission.Permission,
		&rawCreated,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreated, err)
	}

	permission.CreatedAt = createdAt
	return &permission, nil
}
