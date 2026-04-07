package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
)

const auditLogColumns = `a.id, a.instance_id, a.user_id, a.action, a.detail, a.created_at, COALESCE(u.name, ''), COALESCE(u.email, '')`

type Pagination struct {
	Page     int
	PageSize int
}

type AuditLogQuery struct {
	InstanceID *int64
	StartDate  *time.Time
	EndDate    *time.Time
	Actions    []string
	Pagination Pagination
}

type auditLogScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreateAuditLog(log *model.AuditLog) error {
	if log == nil {
		return fmt.Errorf("audit log is nil")
	}
	if strings.TrimSpace(log.Action) == "" {
		return fmt.Errorf("audit action is required")
	}

	detail := strings.TrimSpace(string(log.Detail))
	if detail == "" {
		detail = "null"
	}

	result, err := db.Exec(
		`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		log.InstanceID,
		log.UserID,
		strings.TrimSpace(log.Action),
		detail,
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}

	logID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created audit log id: %w", err)
	}

	created, err := scanAuditLogWithUser(db.QueryRow(
		`SELECT `+auditLogColumns+`
		 FROM audit_logs a
		 LEFT JOIN users u ON u.id = a.user_id
		 WHERE a.id = ?`,
		logID,
	))
	if err != nil {
		return fmt.Errorf("load created audit log: %w", err)
	}

	log.ID = created.ID
	log.InstanceID = created.InstanceID
	log.UserID = created.UserID
	log.Action = created.Action
	log.Detail = created.Detail
	log.CreatedAt = created.CreatedAt
	return nil
}

func (db *DB) ListAuditLogs(query AuditLogQuery) ([]model.AuditLogWithUser, int64, error) {
	whereClauses := make([]string, 0, 4)
	args := make([]any, 0, 8)

	if query.InstanceID != nil {
		whereClauses = append(whereClauses, "a.instance_id = ?")
		args = append(args, *query.InstanceID)
	}
	if query.StartDate != nil {
		whereClauses = append(whereClauses, "unixepoch(a.created_at) >= unixepoch(?)")
		args = append(args, query.StartDate.UTC().Format(time.RFC3339Nano))
	}
	if query.EndDate != nil {
		whereClauses = append(whereClauses, "unixepoch(a.created_at) <= unixepoch(?)")
		args = append(args, query.EndDate.UTC().Format(time.RFC3339Nano))
	}

	actions := normalizeAuditActions(query.Actions)
	if len(actions) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(actions)), ",")
		whereClauses = append(whereClauses, "a.action IN ("+placeholders+")")
		for _, action := range actions {
			args = append(args, action)
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs a`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	limit, offset := normalizeStorePagination(query.Pagination)
	listArgs := append(append([]any(nil), args...), limit, offset)
	rows, err := db.Query(
		`SELECT `+auditLogColumns+`
		 FROM audit_logs a
		 LEFT JOIN users u ON u.id = a.user_id`+whereSQL+`
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]model.AuditLogWithUser, 0, limit)
	for rows.Next() {
		entry, err := scanAuditLogWithUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}

	return logs, total, nil
}

func scanAuditLogWithUser(scanner auditLogScanner) (*model.AuditLogWithUser, error) {
	var (
		entry      model.AuditLogWithUser
		instanceID sql.NullInt64
		userID     sql.NullInt64
		rawDetail  string
		rawCreated string
		userName   string
		userEmail  string
	)

	if err := scanner.Scan(
		&entry.ID,
		&instanceID,
		&userID,
		&entry.Action,
		&rawDetail,
		&rawCreated,
		&userName,
		&userEmail,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreated, err)
	}

	entry.CreatedAt = createdAt
	entry.Detail = normalizeAuditDetail(rawDetail)
	entry.UserName = userName
	entry.UserEmail = userEmail
	if instanceID.Valid {
		entry.InstanceID = &instanceID.Int64
	}
	if userID.Valid {
		entry.UserID = &userID.Int64
	}

	return &entry, nil
}

func normalizeAuditActions(actions []string) []string {
	normalized := make([]string, 0, len(actions))
	for _, action := range actions {
		trimmed := strings.TrimSpace(action)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeStorePagination(p Pagination) (int, int) {
	page := p.Page
	if page <= 0 {
		page = 1
	}
	pageSize := p.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	return pageSize, (page - 1) * pageSize
}

func normalizeAuditDetail(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return json.RawMessage("null")
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(encoded)
}
