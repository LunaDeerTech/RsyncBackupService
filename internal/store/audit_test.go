package store

import (
	"encoding/json"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
)

func TestListAuditLogsFiltersPaginationAndUserJoin(t *testing.T) {
	db := newTestDB(t)
	admin := createAuditTestUser(t, db, "admin@example.com", "Admin")
	viewer := createAuditTestUser(t, db, "viewer@example.com", "Viewer")
	firstInstance := createAuditTestInstance(t, db, "alpha")
	secondInstance := createAuditTestInstance(t, db, "beta")

	insertAuditTestLog(t, db, &firstInstance.ID, &admin.ID, "instance.create", `{"name":"alpha"}`, time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC))
	insertAuditTestLog(t, db, &firstInstance.ID, &viewer.ID, "backup.trigger", `{"backup_id":11}`, time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC))
	insertAuditTestLog(t, db, &secondInstance.ID, &admin.ID, "instance.update", `{"name":"beta"}`, time.Date(2026, 4, 7, 11, 30, 0, 0, time.UTC))
	insertAuditTestLog(t, db, nil, nil, "system.config.update", `{"registration_enabled":false}`, time.Date(2026, 4, 8, 8, 0, 0, 0, time.UTC))

	start := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 7, 23, 59, 59, 0, time.UTC)
	logs, total, err := db.ListAuditLogs(AuditLogQuery{
		StartDate: &start,
		EndDate:   &end,
		Actions:   []string{"backup.trigger", "instance.update"},
		Pagination: Pagination{
			Page:     1,
			PageSize: 1,
		},
	})
	if err != nil {
		t.Fatalf("ListAuditLogs() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("ListAuditLogs() total = %d, want %d", total, 2)
	}
	if len(logs) != 1 {
		t.Fatalf("len(ListAuditLogs()) = %d, want %d", len(logs), 1)
	}
	if logs[0].Action != "instance.update" {
		t.Fatalf("logs[0].Action = %q, want %q", logs[0].Action, "instance.update")
	}
	if logs[0].UserName != "Admin" || logs[0].UserEmail != "admin@example.com" {
		t.Fatalf("logs[0] user = (%q, %q), want (Admin, admin@example.com)", logs[0].UserName, logs[0].UserEmail)
	}
	if string(logs[0].Detail) != `{"name":"beta"}` {
		t.Fatalf("logs[0].Detail = %s, want beta payload", string(logs[0].Detail))
	}

	pageTwo, totalPageTwo, err := db.ListAuditLogs(AuditLogQuery{
		StartDate: &start,
		EndDate:   &end,
		Actions:   []string{"backup.trigger", "instance.update"},
		Pagination: Pagination{
			Page:     2,
			PageSize: 1,
		},
	})
	if err != nil {
		t.Fatalf("ListAuditLogs(page two) error = %v", err)
	}
	if totalPageTwo != 2 {
		t.Fatalf("ListAuditLogs(page two) total = %d, want %d", totalPageTwo, 2)
	}
	if len(pageTwo) != 1 || pageTwo[0].Action != "backup.trigger" {
		t.Fatalf("page two logs = %+v, want single backup.trigger entry", pageTwo)
	}
}

func TestListAuditLogsFiltersByInstance(t *testing.T) {
	db := newTestDB(t)
	admin := createAuditTestUser(t, db, "admin@example.com", "Admin")
	firstInstance := createAuditTestInstance(t, db, "alpha")
	secondInstance := createAuditTestInstance(t, db, "beta")

	insertAuditTestLog(t, db, &firstInstance.ID, &admin.ID, "instance.create", `{"name":"alpha"}`, time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC))
	insertAuditTestLog(t, db, &firstInstance.ID, &admin.ID, "policy.create", `{"name":"daily"}`, time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC))
	insertAuditTestLog(t, db, &secondInstance.ID, &admin.ID, "instance.update", `{"name":"beta"}`, time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC))

	logs, total, err := db.ListAuditLogs(AuditLogQuery{
		InstanceID: &firstInstance.ID,
		Pagination: Pagination{Page: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("ListAuditLogs(instance) error = %v", err)
	}
	if total != 2 {
		t.Fatalf("ListAuditLogs(instance) total = %d, want %d", total, 2)
	}
	if len(logs) != 2 {
		t.Fatalf("len(ListAuditLogs(instance)) = %d, want %d", len(logs), 2)
	}
	for _, entry := range logs {
		if entry.InstanceID == nil || *entry.InstanceID != firstInstance.ID {
			t.Fatalf("entry.InstanceID = %v, want %d", entry.InstanceID, firstInstance.ID)
		}
	}
}

func createAuditTestUser(t *testing.T, db *DB, email, name string) *model.User {
	t.Helper()

	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: "hash",
		Role:         "admin",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(%q) error = %v", email, err)
	}
	return user
}

func createAuditTestInstance(t *testing.T, db *DB, name string) *model.Instance {
	t.Helper()

	instance := &model.Instance{
		Name:       name,
		SourceType: "local",
		SourcePath: "/data/" + name,
		Status:     "idle",
	}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance(%q) error = %v", name, err)
	}
	return instance
}

func insertAuditTestLog(t *testing.T, db *DB, instanceID, userID *int64, action, detail string, createdAt time.Time) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (?, ?, ?, ?, ?)`,
		instanceID,
		userID,
		action,
		detail,
		createdAt.Format("2006-01-02 15:04:05"),
	); err != nil {
		t.Fatalf("insert audit log error = %v", err)
	}
}

func TestCreateAuditLogNormalizesInvalidStoredDetail(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (NULL, NULL, ?, ?, CURRENT_TIMESTAMP)`, "instance.create", "seed"); err != nil {
		t.Fatalf("insert raw audit log error = %v", err)
	}

	logs, total, err := db.ListAuditLogs(AuditLogQuery{Pagination: Pagination{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("ListAuditLogs() error = %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("ListAuditLogs() = (%d, %d), want (1, 1)", total, len(logs))
	}
	var decoded string
	if err := json.Unmarshal(logs[0].Detail, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(detail) error = %v", err)
	}
	if decoded != "seed" {
		t.Fatalf("decoded detail = %q, want %q", decoded, "seed")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action = ?`, "instance.create").Scan(&count); err != nil {
		t.Fatalf("count audit logs error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want %d", count, 1)
	}
}
