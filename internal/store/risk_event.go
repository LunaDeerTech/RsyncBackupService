package store

import (
	"database/sql"
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
)

const riskEventColumns = `id, instance_id, target_id, severity, source, message, resolved, created_at, resolved_at`

type RiskEventQuery struct {
	InstanceID *int64
	TargetID   *int64
	Source     string
	Resolved   *bool
	Limit      int
	Offset     int
}

type riskEventScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CreateRiskEvent(event *model.RiskEvent) error {
	if event == nil {
		return fmt.Errorf("risk event is nil")
	}

	result, err := db.Exec(
		`INSERT INTO risk_events (instance_id, target_id, severity, source, message, resolved, created_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
		event.InstanceID,
		event.TargetID,
		strings.TrimSpace(event.Severity),
		strings.TrimSpace(event.Source),
		strings.TrimSpace(event.Message),
		event.Resolved,
		event.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("create risk event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read created risk event id: %w", err)
	}

	created, err := scanRiskEvent(db.QueryRow(`SELECT `+riskEventColumns+` FROM risk_events WHERE id = ?`, id))
	if err != nil {
		return fmt.Errorf("load created risk event %d: %w", id, err)
	}

	*event = *created
	return nil
}

func (db *DB) ListRiskEvents(query RiskEventQuery) ([]model.RiskEvent, int64, error) {
	filters, args := buildRiskEventFilters(query)

	countSQL := `SELECT COUNT(*) FROM risk_events` + filters
	var total int64
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count risk events: %w", err)
	}

	listArgs := append([]any(nil), args...)
	listSQL := `SELECT ` + riskEventColumns + ` FROM risk_events` + filters + ` ORDER BY created_at DESC, id DESC`
	if query.Limit > 0 {
		listSQL += ` LIMIT ?`
		listArgs = append(listArgs, query.Limit)
		if query.Offset > 0 {
			listSQL += ` OFFSET ?`
			listArgs = append(listArgs, query.Offset)
		}
	} else if query.Offset > 0 {
		listSQL += ` LIMIT -1 OFFSET ?`
		listArgs = append(listArgs, query.Offset)
	}

	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list risk events: %w", err)
	}
	defer rows.Close()

	events := make([]model.RiskEvent, 0)
	for rows.Next() {
		event, err := scanRiskEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan risk event: %w", err)
		}
		events = append(events, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate risk events: %w", err)
	}

	return events, total, nil
}

func (db *DB) GetActiveRiskEvent(instanceID *int64, targetID *int64, source string) (*model.RiskEvent, error) {
	trimmedSource := strings.TrimSpace(source)
	if trimmedSource == "" {
		return nil, fmt.Errorf("risk source is required")
	}

	query := `SELECT ` + riskEventColumns + `
		FROM risk_events
		WHERE resolved = 0 AND source = ?`
	args := []any{trimmedSource}
	if instanceID == nil {
		query += ` AND instance_id IS NULL`
	} else {
		query += ` AND instance_id = ?`
		args = append(args, *instanceID)
	}
	if targetID == nil {
		query += ` AND target_id IS NULL`
	} else {
		query += ` AND target_id = ?`
		args = append(args, *targetID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT 1`

	event, err := scanRiskEvent(db.QueryRow(query, args...))
	if err != nil {
		return nil, fmt.Errorf("get active risk event %q: %w", trimmedSource, err)
	}

	return event, nil
}

func (db *DB) ResolveRiskEvent(id int64) error {
	result, err := db.Exec(
		`UPDATE risk_events
		 SET resolved = 1, resolved_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND resolved = 0`,
		id,
	)
	if err != nil {
		return fmt.Errorf("resolve risk event %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read resolve result for risk event %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("resolve risk event %d: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) UpdateRiskEventSeverity(id int64, severity string) error {
	result, err := db.Exec(`UPDATE risk_events SET severity = ? WHERE id = ?`, strings.TrimSpace(severity), id)
	if err != nil {
		return fmt.Errorf("update risk event %d severity: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result for risk event %d severity: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("update risk event %d severity: %w", id, sql.ErrNoRows)
	}

	return nil
}

func (db *DB) ListUnresolvedRiskEvents() ([]model.RiskEvent, error) {
	resolved := false
	events, _, err := db.ListRiskEvents(RiskEventQuery{Resolved: &resolved})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (db *DB) CountConsecutiveFailures(instanceID int64, policyID int64) (int, error) {
	rows, err := db.Query(
		`SELECT status
		 FROM backups
		 WHERE instance_id = ? AND policy_id = ? AND status IN ('success', 'failed')
		 ORDER BY COALESCE(completed_at, started_at, created_at) DESC, id DESC`,
		instanceID,
		policyID,
	)
	if err != nil {
		return 0, fmt.Errorf("count consecutive failures for instance %d policy %d: %w", instanceID, policyID, err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, fmt.Errorf("scan consecutive failure status for instance %d policy %d: %w", instanceID, policyID, err)
		}
		if strings.EqualFold(status, "failed") {
			count++
			continue
		}
		break
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate consecutive failures for instance %d policy %d: %w", instanceID, policyID, err)
	}

	return count, nil
}

func buildRiskEventFilters(query RiskEventQuery) (string, []any) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if query.InstanceID != nil {
		conditions = append(conditions, "instance_id = ?")
		args = append(args, *query.InstanceID)
	}
	if query.TargetID != nil {
		conditions = append(conditions, "target_id = ?")
		args = append(args, *query.TargetID)
	}
	if trimmed := strings.TrimSpace(query.Source); trimmed != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, trimmed)
	}
	if query.Resolved != nil {
		conditions = append(conditions, "resolved = ?")
		args = append(args, *query.Resolved)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func scanRiskEvent(scanner riskEventScanner) (*model.RiskEvent, error) {
	var (
		event      model.RiskEvent
		instanceID sql.NullInt64
		targetID   sql.NullInt64
		rawCreated string
		rawResolve sql.NullString
	)

	if err := scanner.Scan(
		&event.ID,
		&instanceID,
		&targetID,
		&event.Severity,
		&event.Source,
		&event.Message,
		&event.Resolved,
		&rawCreated,
		&rawResolve,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse risk event created_at %q: %w", rawCreated, err)
	}
	event.CreatedAt = createdAt
	if instanceID.Valid {
		event.InstanceID = &instanceID.Int64
	}
	if targetID.Valid {
		event.TargetID = &targetID.Int64
	}
	if rawResolve.Valid {
		resolvedAt, err := parseSQLiteTime(rawResolve.String)
		if err != nil {
			return nil, fmt.Errorf("parse risk event resolved_at %q: %w", rawResolve.String, err)
		}
		event.ResolvedAt = &resolvedAt
	}

	return &event, nil
}
