package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (db *DB) GetSystemConfig(key string) (string, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", fmt.Errorf("system config key is required")
	}

	var value string
	if err := db.QueryRow(`SELECT value FROM system_configs WHERE key = ?`, trimmedKey).Scan(&value); err != nil {
		return "", fmt.Errorf("get system config %q: %w", trimmedKey, err)
	}

	return value, nil
}

func (db *DB) GetSystemConfigs(keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		value, err := db.GetSystemConfig(trimmedKey)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			continue
		}
		values[trimmedKey] = value
	}

	return values, nil
}

func (db *DB) SetSystemConfig(key, value string) error {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return fmt.Errorf("system config key is required")
	}

	if _, err := db.Exec(
		`INSERT INTO system_configs (key, value, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		trimmedKey,
		value,
	); err != nil {
		return fmt.Errorf("set system config %q: %w", trimmedKey, err)
	}

	return nil
}

func (db *DB) SetSystemConfigs(values map[string]string) error {
	if len(values) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin set system configs: %w", err)
	}
	defer tx.Rollback()

	keys := make([]string, 0, len(values))
	for key := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return fmt.Errorf("system config key is required")
		}
		keys = append(keys, trimmedKey)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := tx.Exec(
			`INSERT INTO system_configs (key, value, updated_at)
			 VALUES (?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
			key,
			values[key],
		); err != nil {
			return fmt.Errorf("set system config %q: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit system configs: %w", err)
	}

	return nil
}