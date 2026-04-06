package store

import (
	"database/sql"
	"errors"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestInstancePermissionsSetGetAndList(t *testing.T) {
	db := newTestDB(t)
	admin := createStoreTestUser(t, db, "admin@example.com", "admin")
	viewer := createStoreTestUser(t, db, "viewer@example.com", "viewer")
	firstInstanceID := createStoreTestInstance(t, db, "instance-a")
	secondInstanceID := createStoreTestInstance(t, db, "instance-b")

	if err := db.SetInstancePermissions(firstInstanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}, {
		UserID:     admin.ID,
		Permission: "manage",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions(first) error = %v", err)
	}
	if err := db.SetInstancePermissions(secondInstanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions(second) error = %v", err)
	}

	permission, err := db.GetInstancePermission(viewer.ID, firstInstanceID)
	if err != nil {
		t.Fatalf("GetInstancePermission() error = %v", err)
	}
	if permission.UserID != viewer.ID || permission.InstanceID != firstInstanceID {
		t.Fatalf("permission = %+v, want viewer/%d permission", permission, firstInstanceID)
	}
	if permission.Permission != "readonly" {
		t.Fatalf("permission.Permission = %q, want %q", permission.Permission, "readonly")
	}
	if permission.CreatedAt.IsZero() {
		t.Fatal("permission.CreatedAt is zero")
	}

	permissions, err := db.ListInstancePermissionsByUser(viewer.ID)
	if err != nil {
		t.Fatalf("ListInstancePermissionsByUser() error = %v", err)
	}
	if len(permissions) != 2 {
		t.Fatalf("ListInstancePermissionsByUser() len = %d, want %d", len(permissions), 2)
	}
	if permissions[0].InstanceID != firstInstanceID || permissions[1].InstanceID != secondInstanceID {
		t.Fatalf("ListInstancePermissionsByUser() instance order = [%d %d], want [%d %d]", permissions[0].InstanceID, permissions[1].InstanceID, firstInstanceID, secondInstanceID)
	}
}

func TestSetInstancePermissionsReplacesExistingRows(t *testing.T) {
	db := newTestDB(t)
	viewer := createStoreTestUser(t, db, "viewer@example.com", "viewer")
	admin := createStoreTestUser(t, db, "admin@example.com", "admin")
	instanceID := createStoreTestInstance(t, db, "instance-a")

	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions(initial) error = %v", err)
	}
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{
		UserID:     admin.ID,
		Permission: "manage",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions(replace) error = %v", err)
	}

	if _, err := db.GetInstancePermission(viewer.ID, instanceID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetInstancePermission(old user) error = %v, want sql.ErrNoRows", err)
	}

	permission, err := db.GetInstancePermission(admin.ID, instanceID)
	if err != nil {
		t.Fatalf("GetInstancePermission(new user) error = %v", err)
	}
	if permission.Permission != "manage" {
		t.Fatalf("permission.Permission = %q, want %q", permission.Permission, "manage")
	}

	viewerPermissions, err := db.ListInstancePermissionsByUser(viewer.ID)
	if err != nil {
		t.Fatalf("ListInstancePermissionsByUser(viewer) error = %v", err)
	}
	if len(viewerPermissions) != 0 {
		t.Fatalf("ListInstancePermissionsByUser(viewer) len = %d, want 0", len(viewerPermissions))
	}
}

func createStoreTestUser(t *testing.T, db *DB, email string, role string) *model.User {
	t.Helper()

	user := &model.User{
		Email:        email,
		Name:         email,
		PasswordHash: "hash",
		Role:         role,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(%q) error = %v", email, err)
	}

	return user
}

func createStoreTestInstance(t *testing.T, db *DB, name string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		name,
		"local",
		"/data/"+name,
		"idle",
	)
	if err != nil {
		t.Fatalf("insert instance %q error = %v", name, err)
	}

	instanceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error = %v", err)
	}

	return instanceID
}
