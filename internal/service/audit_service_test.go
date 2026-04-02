package service

import (
	"context"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

func TestAuditServiceListFiltersAndPaginates(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)
	viewer := createAuthServiceTestUser(t, fixture.db, "auditor", "auditor-secret", false)

	baseTime := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	logs := []model.AuditLog{
		{UserID: fixture.admin.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 1, Detail: `{"seq":1}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(1 * time.Minute)},
		{UserID: fixture.admin.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 2, Detail: `{"seq":2}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(2 * time.Minute)},
		{UserID: fixture.admin.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 3, Detail: `{"seq":3}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(3 * time.Minute)},
		{UserID: fixture.admin.ID, Action: "users.create", ResourceType: "users", ResourceID: 4, Detail: `{"seq":4}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(4 * time.Minute)},
		{UserID: viewer.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 5, Detail: `{"seq":5}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(5 * time.Minute)},
	}
	if err := fixture.db.Create(&logs).Error; err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	svc := NewAuditService(fixture.db)
	startTime := baseTime.Add(30 * time.Second)
	endTime := baseTime.Add(4 * time.Minute)
	logsPage, total, err := svc.List(context.Background(), ListAuditLogsRequest{
		UserID:    &fixture.admin.ID,
		Action:    "instances.create",
		StartTime: &startTime,
		EndTime:   &endTime,
		Page:      2,
		PageSize:  1,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 matching audit logs, got %d", total)
	}
	if len(logsPage) != 1 {
		t.Fatalf("expected 1 audit log on the requested page, got %d", len(logsPage))
	}
	if logsPage[0].ResourceID != 2 {
		t.Fatalf("expected second page to return resource_id 2, got %d", logsPage[0].ResourceID)
	}
	if logsPage[0].User.Username != fixture.admin.Username {
		t.Fatalf("expected preloaded username %q, got %q", fixture.admin.Username, logsPage[0].User.Username)
	}
}