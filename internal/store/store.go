package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

const latestSchemaVersion = 2

type DB struct {
	*sql.DB
}

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{
	{
		version: 1,
		statements: []string{
			systemConfigsDDL,
			`CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				email TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				password_hash TEXT NOT NULL,
				role TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS remote_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				type TEXT NOT NULL,
				host TEXT NOT NULL,
				port INTEGER NOT NULL,
				username TEXT NOT NULL,
				private_key_path TEXT NOT NULL,
				cloud_provider TEXT,
				cloud_config TEXT,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS instances (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				source_type TEXT NOT NULL,
				source_path TEXT NOT NULL,
				remote_config_id INTEGER,
				status TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (remote_config_id) REFERENCES remote_configs(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS backup_targets (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				backup_type TEXT NOT NULL,
				storage_type TEXT NOT NULL,
				storage_path TEXT NOT NULL,
				remote_config_id INTEGER,
				total_capacity_bytes INTEGER,
				used_capacity_bytes INTEGER,
				last_health_check DATETIME,
				health_status TEXT NOT NULL,
				health_message TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (remote_config_id) REFERENCES remote_configs(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS policies (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				instance_id INTEGER NOT NULL,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				target_id INTEGER NOT NULL,
				schedule_type TEXT NOT NULL,
				schedule_value TEXT NOT NULL,
				enabled BOOLEAN NOT NULL,
				compression BOOLEAN NOT NULL,
				encryption BOOLEAN NOT NULL,
				encryption_key_hash TEXT,
				split_enabled BOOLEAN NOT NULL,
				split_size_mb INTEGER,
				retention_type TEXT NOT NULL,
				retention_value INTEGER NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE,
				FOREIGN KEY (target_id) REFERENCES backup_targets(id)
			)`,
			`CREATE TABLE IF NOT EXISTS backups (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				instance_id INTEGER NOT NULL,
				policy_id INTEGER NOT NULL,
				type TEXT NOT NULL,
				status TEXT NOT NULL,
				snapshot_path TEXT NOT NULL,
				backup_size_bytes INTEGER NOT NULL,
				actual_size_bytes INTEGER NOT NULL,
				started_at DATETIME,
				completed_at DATETIME,
				duration_seconds INTEGER NOT NULL,
				error_message TEXT NOT NULL,
				rsync_stats TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE,
				FOREIGN KEY (policy_id) REFERENCES policies(id)
			)`,
			`CREATE TABLE IF NOT EXISTS tasks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				instance_id INTEGER NOT NULL,
				backup_id INTEGER,
				type TEXT NOT NULL,
				status TEXT NOT NULL,
				progress INTEGER NOT NULL,
				current_step TEXT NOT NULL,
				started_at DATETIME,
				completed_at DATETIME,
				estimated_end DATETIME,
				error_message TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE,
				FOREIGN KEY (backup_id) REFERENCES backups(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS instance_permissions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				instance_id INTEGER NOT NULL,
				permission TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(user_id, instance_id),
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS audit_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				instance_id INTEGER,
				user_id INTEGER,
				action TEXT NOT NULL,
				detail TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE SET NULL,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS risk_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				instance_id INTEGER,
				target_id INTEGER,
				severity TEXT NOT NULL,
				source TEXT NOT NULL,
				message TEXT NOT NULL,
				resolved BOOLEAN NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				resolved_at DATETIME,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE SET NULL,
				FOREIGN KEY (target_id) REFERENCES backup_targets(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS notification_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				instance_id INTEGER NOT NULL,
				enabled BOOLEAN NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
			)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`ALTER TABLE backups ADD COLUMN trigger_source TEXT NOT NULL DEFAULT 'manual'`,
		},
	},
}

const systemConfigsDDL = `CREATE TABLE IF NOT EXISTS system_configs (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

func New(dataDir string) (*DB, error) {
	databasePath := filepath.Join(dataDir, "rbs.db")
	sqlDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	var journalMode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode=WAL;").Scan(&journalMode); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable wal mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		sqlDB.Close()
		return nil, fmt.Errorf("unexpected journal mode %q", journalMode)
	}

	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return &DB{DB: sqlDB}, nil
}

func (db *DB) Migrate() error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(systemConfigsDDL); err != nil {
		return fmt.Errorf("ensure system_configs table: %w", err)
	}

	currentVersion, err := currentSchemaVersion(tx)
	if err != nil {
		return err
	}
	if currentVersion > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", currentVersion, latestSchemaVersion)
	}

	for _, migration := range migrations {
		if migration.version <= currentVersion {
			continue
		}

		for _, statement := range migration.statements {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("apply schema version %d: %w", migration.version, err)
			}
		}

		if err := setSchemaVersion(tx, migration.version); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	return nil
}

func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func currentSchemaVersion(tx *sql.Tx) (int, error) {
	var rawVersion string
	err := tx.QueryRow(`SELECT value FROM system_configs WHERE key = 'schema_version'`).Scan(&rawVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load schema version: %w", err)
	}

	version, err := strconv.Atoi(rawVersion)
	if err != nil {
		return 0, fmt.Errorf("parse schema version %q: %w", rawVersion, err)
	}

	return version, nil
}

func setSchemaVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(
		`INSERT INTO system_configs (key, value, updated_at)
		 VALUES ('schema_version', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		strconv.Itoa(version),
	)
	if err != nil {
		return fmt.Errorf("store schema version %d: %w", version, err)
	}

	return nil
}
