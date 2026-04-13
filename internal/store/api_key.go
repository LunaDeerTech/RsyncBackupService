package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
)

const apiKeyColumns = `id, user_id, name, key_prefix, key_hash, last_used_at, created_at`

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreateAPIKey(apiKey *model.APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("api key is nil")
	}
	if apiKey.UserID <= 0 {
		return fmt.Errorf("api key user id is required")
	}
	if strings.TrimSpace(apiKey.Name) == "" {
		return fmt.Errorf("api key name is required")
	}
	if strings.TrimSpace(apiKey.KeyPrefix) == "" {
		return fmt.Errorf("api key prefix is required")
	}
	if strings.TrimSpace(apiKey.KeyHash) == "" {
		return fmt.Errorf("api key hash is required")
	}

	result, err := db.Exec(
		`INSERT INTO api_keys (user_id, name, key_prefix, key_hash, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		apiKey.UserID,
		strings.TrimSpace(apiKey.Name),
		strings.TrimSpace(apiKey.KeyPrefix),
		strings.TrimSpace(apiKey.KeyHash),
	)
	if err != nil {
		return fmt.Errorf("create api key for user %d: %w", apiKey.UserID, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created api key id: %w", err)
	}

	created, err := db.GetAPIKeyByID(id)
	if err != nil {
		return fmt.Errorf("load created api key %d: %w", id, err)
	}
	created.Key = apiKey.Key
	*apiKey = *created
	return nil
}

func (db *DB) GetAPIKeyByID(id int64) (*model.APIKey, error) {
	apiKey, err := scanAPIKey(db.QueryRow(`SELECT `+apiKeyColumns+` FROM api_keys WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get api key by id %d: %w", id, err)
	}

	return apiKey, nil
}

func (db *DB) GetAPIKeyByHash(hash string) (*model.APIKey, error) {
	apiKey, err := scanAPIKey(db.QueryRow(`SELECT `+apiKeyColumns+` FROM api_keys WHERE key_hash = ?`, strings.TrimSpace(hash)))
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}

	return apiKey, nil
}

func (db *DB) ListAPIKeysByUser(userID int64) ([]model.APIKey, error) {
	rows, err := db.Query(
		`SELECT `+apiKeyColumns+`
		 FROM api_keys
		 WHERE user_id = ?
		 ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys by user %d: %w", userID, err)
	}
	defer rows.Close()

	apiKeys := make([]model.APIKey, 0)
	for rows.Next() {
		apiKey, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scan api key by user %d: %w", userID, err)
		}
		apiKeys = append(apiKeys, *apiKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys by user %d: %w", userID, err)
	}

	return apiKeys, nil
}

func (db *DB) DeleteAPIKeyByIDAndUser(id, userID int64) error {
	result, err := db.Exec(`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete api key %d for user %d: %w", id, userID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for api key %d user %d: %w", id, userID, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete api key %d for user %d: %w", id, userID, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) TouchAPIKeyLastUsed(id int64) error {
	result, err := db.Exec(`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("touch api key %d last used: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read touch result for api key %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("touch api key %d last used: %w", id, sql.ErrNoRows)
	}

	return nil
}

func scanAPIKey(scanner apiKeyScanner) (*model.APIKey, error) {
	var (
		apiKey     model.APIKey
		lastUsedAt sql.NullString
		rawCreated string
	)

	if err := scanner.Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.KeyPrefix,
		&apiKey.KeyHash,
		&lastUsedAt,
		&rawCreated,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreated, err)
	}
	apiKey.CreatedAt = createdAt

	if lastUsedAt.Valid {
		parsedLastUsedAt, err := parseSQLiteTime(lastUsedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse last_used_at %q: %w", lastUsedAt.String, err)
		}
		apiKey.LastUsedAt = &parsedLastUsedAt
	}

	return &apiKey, nil
}

func apiKeyTimeOrNil(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.UTC()
}
