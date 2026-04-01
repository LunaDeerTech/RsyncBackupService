package service

import (
	"context"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

func TestSetInstanceRoleUpsertsSingleRecord(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)
	managedUser := createAuthServiceTestUser(t, fixture.db, "viewer", "viewer-secret", false)
	permissionService := NewPermissionService(fixture.db)

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	permission, err := permissionService.SetInstanceRole(context.Background(), instance.ID, managedUser.ID, RoleViewer)
	if err != nil {
		t.Fatalf("set viewer role: %v", err)
	}
	if permission.Role != RoleViewer {
		t.Fatalf("expected viewer role, got %q", permission.Role)
	}

	permission, err = permissionService.SetInstanceRole(context.Background(), instance.ID, managedUser.ID, RoleAdmin)
	if err != nil {
		t.Fatalf("upgrade role: %v", err)
	}
	if permission.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %q", permission.Role)
	}

	var permissionCount int64
	if err := fixture.db.Model(&model.InstancePermission{}).Where("user_id = ? AND instance_id = ?", managedUser.ID, instance.ID).Count(&permissionCount).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if permissionCount != 1 {
		t.Fatalf("expected one permission row after upsert, got %d", permissionCount)
	}
}