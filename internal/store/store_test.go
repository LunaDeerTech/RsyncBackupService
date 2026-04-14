package store

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewAndMigrateCreatesSchema(t *testing.T) {
	dataDir := t.TempDir()

	db, err := New(dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() second run error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "rbs.db")); err != nil {
		t.Fatalf("database file missing: %v", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("journal mode query error = %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want %q", journalMode, "wal")
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("busy timeout query error = %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy timeout = %d, want %d", busyTimeout, 5000)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("foreign keys query error = %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign keys = %d, want %d", foreignKeys, 1)
	}

	if db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("max open connections = %d, want %d", db.Stats().MaxOpenConnections, 1)
	}

	expectedTables := []string{
		"api_keys",
		"audit_logs",
		"backup_targets",
		"backups",
		"instance_permissions",
		"instances",
		"notification_subscriptions",
		"policies",
		"remote_configs",
		"risk_events",
		"system_configs",
		"tasks",
		"users",
	}

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables error = %v", err)
	}
	defer rows.Close()

	var actualTables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			t.Fatalf("scan table name error = %v", err)
		}
		actualTables = append(actualTables, tableName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables error = %v", err)
	}

	sort.Strings(actualTables)
	if len(actualTables) != len(expectedTables) {
		t.Fatalf("table count = %d, want %d (%v)", len(actualTables), len(expectedTables), actualTables)
	}
	for index, expected := range expectedTables {
		if actualTables[index] != expected {
			t.Fatalf("table[%d] = %q, want %q", index, actualTables[index], expected)
		}
	}

	var schemaVersion string
	if err := db.QueryRow(`SELECT value FROM system_configs WHERE key = 'schema_version'`).Scan(&schemaVersion); err != nil {
		t.Fatalf("query schema version error = %v", err)
	}
	if schemaVersion != "8" {
		t.Fatalf("schema version = %q, want %q", schemaVersion, "8")
	}

	backupColumns := make(map[string]struct{})
	columnRows, err := db.Query(`PRAGMA table_info(backups)`)
	if err != nil {
		t.Fatalf("query backup table info error = %v", err)
	}
	defer columnRows.Close()
	for columnRows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := columnRows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("scan backup table info error = %v", err)
		}
		backupColumns[name] = struct{}{}
	}
	if err := columnRows.Err(); err != nil {
		t.Fatalf("iterate backup table info error = %v", err)
	}
	if _, ok := backupColumns["retry_root_backup_id"]; !ok {
		t.Fatal("backups table missing retry_root_backup_id column")
	}
}
