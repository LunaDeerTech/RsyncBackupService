package store

import (
	"database/sql"
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
)

const remoteConfigColumns = `id, name, type, host, port, username, private_key_path, cloud_provider, cloud_config, created_at, updated_at`

type RemoteConfigUsage struct {
	Instances     []string `json:"instances,omitempty"`
	BackupTargets []string `json:"backup_targets,omitempty"`
}

type remoteConfigScanner interface {
	Scan(dest ...any) error
}

func (u RemoteConfigUsage) InUse() bool {
	return len(u.Instances) > 0 || len(u.BackupTargets) > 0
}

func (db *DB) CreateRemoteConfig(remote *model.RemoteConfig) error {
	if remote == nil {
		return fmt.Errorf("remote config is nil")
	}

	result, err := db.Exec(
		`INSERT INTO remote_configs (name, type, host, port, username, private_key_path, cloud_provider, cloud_config, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		remote.Name,
		remote.Type,
		remote.Host,
		remote.Port,
		remote.Username,
		remote.PrivateKeyPath,
		remote.CloudProvider,
		remote.CloudConfig,
	)
	if err != nil {
		return fmt.Errorf("create remote config: %w", err)
	}

	remoteID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created remote config id: %w", err)
	}

	created, err := db.GetRemoteConfigByID(remoteID)
	if err != nil {
		return fmt.Errorf("load created remote config: %w", err)
	}

	*remote = *created
	return nil
}

func (db *DB) GetRemoteConfigByID(id int64) (*model.RemoteConfig, error) {
	remote, err := scanRemoteConfig(db.QueryRow(`SELECT `+remoteConfigColumns+` FROM remote_configs WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get remote config by id %d: %w", id, err)
	}

	return remote, nil
}

func (db *DB) GetRemoteConfigByName(name string) (*model.RemoteConfig, error) {
	remote, err := scanRemoteConfig(db.QueryRow(`SELECT `+remoteConfigColumns+` FROM remote_configs WHERE name = ?`, strings.TrimSpace(name)))
	if err != nil {
		return nil, fmt.Errorf("get remote config by name %q: %w", name, err)
	}

	return remote, nil
}

func (db *DB) ListRemoteConfigs() ([]model.RemoteConfig, error) {
	rows, err := db.Query(`SELECT ` + remoteConfigColumns + ` FROM remote_configs ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list remote configs: %w", err)
	}
	defer rows.Close()

	remotes := make([]model.RemoteConfig, 0)
	for rows.Next() {
		remote, err := scanRemoteConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed remote config: %w", err)
		}
		remotes = append(remotes, *remote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remote configs: %w", err)
	}

	return remotes, nil
}

func (db *DB) ListRemoteConfigsPage(limit, offset int) ([]model.RemoteConfig, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list remote configs page: limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list remote configs page: offset must be non-negative")
	}

	rows, err := db.Query(
		`SELECT `+remoteConfigColumns+` FROM remote_configs ORDER BY id ASC LIMIT ? OFFSET ?`,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list remote configs page limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	remotes := make([]model.RemoteConfig, 0, limit)
	for rows.Next() {
		remote, err := scanRemoteConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed remote config page: %w", err)
		}
		remotes = append(remotes, *remote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remote config page: %w", err)
	}

	return remotes, nil
}

func (db *DB) CountRemoteConfigs() (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM remote_configs`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count remote configs: %w", err)
	}

	return count, nil
}

func (db *DB) UpdateRemoteConfig(remote *model.RemoteConfig) error {
	if remote == nil {
		return fmt.Errorf("remote config is nil")
	}

	result, err := db.Exec(
		`UPDATE remote_configs
		 SET name = ?, type = ?, host = ?, port = ?, username = ?, private_key_path = ?, cloud_provider = ?, cloud_config = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		remote.Name,
		remote.Type,
		remote.Host,
		remote.Port,
		remote.Username,
		remote.PrivateKeyPath,
		remote.CloudProvider,
		remote.CloudConfig,
		remote.ID,
	)
	if err != nil {
		return fmt.Errorf("update remote config %d: %w", remote.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for remote config %d: %w", remote.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update remote config %d: %w", remote.ID, sql.ErrNoRows)
	}

	updated, err := db.GetRemoteConfigByID(remote.ID)
	if err != nil {
		return fmt.Errorf("load updated remote config %d: %w", remote.ID, err)
	}

	*remote = *updated
	return nil
}

func (db *DB) DeleteRemoteConfig(id int64) error {
	result, err := db.Exec(`DELETE FROM remote_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete remote config %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for remote config %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete remote config %d: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) IsRemoteConfigInUse(id int64) (bool, error) {
	usage, err := db.GetRemoteConfigUsage(id)
	if err != nil {
		return false, err
	}

	return usage.InUse(), nil
}

func (db *DB) GetRemoteConfigUsage(id int64) (RemoteConfigUsage, error) {
	instances, err := db.listRemoteConfigReferenceNames(`SELECT name FROM instances WHERE remote_config_id = ? ORDER BY name ASC`, id)
	if err != nil {
		return RemoteConfigUsage{}, fmt.Errorf("list instances using remote config %d: %w", id, err)
	}

	backupTargets, err := db.listRemoteConfigReferenceNames(`SELECT name FROM backup_targets WHERE remote_config_id = ? ORDER BY name ASC`, id)
	if err != nil {
		return RemoteConfigUsage{}, fmt.Errorf("list backup targets using remote config %d: %w", id, err)
	}

	return RemoteConfigUsage{
		Instances:     instances,
		BackupTargets: backupTargets,
	}, nil
}

func (db *DB) listRemoteConfigReferenceNames(query string, id int64) ([]string, error) {
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

func scanRemoteConfig(scanner remoteConfigScanner) (*model.RemoteConfig, error) {
	var (
		remote        model.RemoteConfig
		cloudProvider sql.NullString
		cloudConfig   sql.NullString
		rawCreated    string
		rawUpdated    string
	)

	if err := scanner.Scan(
		&remote.ID,
		&remote.Name,
		&remote.Type,
		&remote.Host,
		&remote.Port,
		&remote.Username,
		&remote.PrivateKeyPath,
		&cloudProvider,
		&cloudConfig,
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

	remote.CreatedAt = createdAt
	remote.UpdatedAt = updatedAt
	if cloudProvider.Valid {
		remote.CloudProvider = &cloudProvider.String
	}
	if cloudConfig.Valid {
		remote.CloudConfig = &cloudConfig.String
	}

	return &remote, nil
}
