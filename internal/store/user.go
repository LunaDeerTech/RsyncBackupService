package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
)

const userColumns = `id, email, name, password_hash, role, created_at, updated_at`

type userScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreateUser(user *model.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	result, err := db.Exec(
		`INSERT INTO users (email, name, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		user.Email,
		user.Name,
		user.PasswordHash,
		user.Role,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created user id: %w", err)
	}

	created, err := db.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("load created user: %w", err)
	}

	*user = *created
	return nil
}

func (db *DB) GetUserByEmail(email string) (*model.User, error) {
	user, err := scanUser(db.QueryRow(`SELECT `+userColumns+` FROM users WHERE email = ?`, email))
	if err != nil {
		return nil, fmt.Errorf("get user by email %q: %w", email, err)
	}

	return user, nil
}

func (db *DB) GetUserByID(id int64) (*model.User, error) {
	user, err := scanUser(db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}

	return user, nil
}

func (db *DB) ListUsers() ([]model.User, error) {
	rows, err := db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed user: %w", err)
		}
		users = append(users, *user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (db *DB) ListUsersPage(limit, offset int) ([]model.User, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list users page: limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list users page: offset must be non-negative")
	}

	rows, err := db.Query(
		`SELECT `+userColumns+` FROM users ORDER BY id ASC LIMIT ? OFFSET ?`,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list users page limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	users := make([]model.User, 0, limit)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed user page: %w", err)
		}
		users = append(users, *user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user page: %w", err)
	}

	return users, nil
}

func (db *DB) UpdateUser(user *model.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	result, err := db.Exec(
		`UPDATE users
		 SET email = ?, name = ?, password_hash = ?, role = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		user.Email,
		user.Name,
		user.PasswordHash,
		user.Role,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user %d: %w", user.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for user %d: %w", user.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update user %d: %w", user.ID, sql.ErrNoRows)
	}

	updated, err := db.GetUserByID(user.ID)
	if err != nil {
		return fmt.Errorf("load updated user %d: %w", user.ID, err)
	}

	*user = *updated
	return nil
}

func (db *DB) DeleteUser(id int64) error {
	result, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for user %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete user %d: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) DeleteUserWithCleanup(id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete user %d: %w", id, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM instance_permissions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete instance permissions for user %d: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM notification_subscriptions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete notification subscriptions for user %d: %w", id, err)
	}

	result, err := tx.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for user %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete user %d: %w", id, sql.ErrNoRows)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user %d: %w", id, err)
	}

	return nil
}

func (db *DB) CountUsers() (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return count, nil
}

func (db *DB) CountUsersByRole(role string) (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, strings.TrimSpace(role)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users by role %q: %w", role, err)
	}

	return count, nil
}

func scanUser(scanner userScanner) (*model.User, error) {
	var (
		user       model.User
		rawCreated string
		rawUpdated string
	)

	if err := scanner.Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&user.Role,
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

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt
	return &user, nil
}

func parseSQLiteTime(raw string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 +0000 UTC",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			if parsed.Location() == time.Local {
				return parsed.UTC(), nil
			}
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, errors.New("unsupported time format")
}
