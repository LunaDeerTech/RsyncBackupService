package store

import (
	"database/sql"
	"fmt"

	"rsync-backup-service/internal/model"
)

type dashboardRiskEventScanner interface {
	Scan(dest ...any) error
}

func (db *DB) CountTasksByStatus(status string) (int, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = ?`, status).Scan(&count); err != nil {
		return 0, fmt.Errorf("count tasks by status %q: %w", status, err)
	}

	return count, nil
}

func (db *DB) CountBackups() (int64, error) {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM backups`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count backups: %w", err)
	}

	return count, nil
}

func (db *DB) CountUnresolvedRiskEvents() (int, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM risk_events WHERE resolved = 0`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unresolved risk events: %w", err)
	}

	return count, nil
}

func (db *DB) CountUnresolvedRiskEventsByInstance() (map[int64]int, error) {
	rows, err := db.Query(
		`SELECT instance_id, COUNT(*)
		 FROM risk_events
		 WHERE resolved = 0 AND instance_id IS NOT NULL
		 GROUP BY instance_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("count unresolved risk events by instance: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var (
			instanceID int64
			count      int
		)
		if err := rows.Scan(&instanceID, &count); err != nil {
			return nil, fmt.Errorf("scan unresolved risk event count by instance: %w", err)
		}
		counts[instanceID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unresolved risk event counts by instance: %w", err)
	}

	return counts, nil
}

func (db *DB) ListDashboardRiskEvents(query RiskEventQuery) ([]model.DashboardRiskEvent, int64, error) {
	filters, args := buildRiskEventFilters(query)

	countSQL := `SELECT COUNT(*) FROM risk_events` + filters
	var total int64
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count dashboard risk events: %w", err)
	}

	listArgs := append([]any(nil), args...)
	listSQL := `SELECT re.id,
	       re.instance_id,
	       COALESCE(i.name, ''),
	       re.target_id,
	       COALESCE(bt.name, ''),
	       re.severity,
	       re.source,
	       re.message,
	       re.resolved,
	       re.created_at,
	       re.resolved_at
	FROM risk_events re
	LEFT JOIN instances i ON i.id = re.instance_id
	LEFT JOIN backup_targets bt ON bt.id = re.target_id` + filters + `
	ORDER BY CASE re.severity
		WHEN 'critical' THEN 3
		WHEN 'warning' THEN 2
		WHEN 'info' THEN 1
		ELSE 0
	END DESC,
	re.created_at DESC,
	re.id DESC`
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
		return nil, 0, fmt.Errorf("list dashboard risk events: %w", err)
	}
	defer rows.Close()

	events := make([]model.DashboardRiskEvent, 0)
	for rows.Next() {
		event, err := scanDashboardRiskEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan dashboard risk event: %w", err)
		}
		events = append(events, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate dashboard risk events: %w", err)
	}

	return events, total, nil
}

func (db *DB) ListDailyBackupResults(days int) ([]model.DailyBackupResult, error) {
	if days <= 0 {
		return []model.DailyBackupResult{}, nil
	}

	startOffset := fmt.Sprintf("-%d days", days-1)
	rows, err := db.Query(
		`WITH RECURSIVE days(day) AS (
			SELECT date('now', ?)
			UNION ALL
			SELECT date(day, '+1 day') FROM days WHERE day < date('now')
		 )
		 SELECT days.day,
		        COALESCE(SUM(CASE WHEN backups.status = 'success' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN backups.status = 'failed' THEN 1 ELSE 0 END), 0)
		 FROM days
		 LEFT JOIN backups
		   ON substr(COALESCE(backups.completed_at, backups.started_at, backups.created_at), 1, 10) = days.day
		 GROUP BY days.day
		 ORDER BY days.day ASC`,
		startOffset,
	)
	if err != nil {
		return nil, fmt.Errorf("list daily backup results for %d days: %w", days, err)
	}
	defer rows.Close()

	results := make([]model.DailyBackupResult, 0, days)
	for rows.Next() {
		var result model.DailyBackupResult
		if err := rows.Scan(&result.Date, &result.Success, &result.Failed); err != nil {
			return nil, fmt.Errorf("scan daily backup result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily backup results: %w", err)
	}

	return results, nil
}

func scanDashboardRiskEvent(scanner dashboardRiskEventScanner) (*model.DashboardRiskEvent, error) {
	var (
		event       model.DashboardRiskEvent
		instanceID  sql.NullInt64
		targetID    sql.NullInt64
		rawCreated  string
		rawResolved sql.NullString
	)

	if err := scanner.Scan(
		&event.ID,
		&instanceID,
		&event.InstanceName,
		&targetID,
		&event.TargetName,
		&event.Severity,
		&event.Source,
		&event.Message,
		&event.Resolved,
		&rawCreated,
		&rawResolved,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse dashboard risk event created_at %q: %w", rawCreated, err)
	}
	event.CreatedAt = createdAt
	if instanceID.Valid {
		event.InstanceID = &instanceID.Int64
	}
	if targetID.Valid {
		event.TargetID = &targetID.Int64
	}
	if rawResolved.Valid {
		resolvedAt, err := parseSQLiteTime(rawResolved.String)
		if err != nil {
			return nil, fmt.Errorf("parse dashboard risk event resolved_at %q: %w", rawResolved.String, err)
		}
		event.ResolvedAt = &resolvedAt
	}

	return &event, nil
}
