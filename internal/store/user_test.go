package store

import (
	"database/sql"
	"errors"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)

	first := &model.User{
		Email:        "first@example.com",
		Name:         "first",
		PasswordHash: "hash-1",
		Role:         "admin",
	}
	if err := db.CreateUser(first); err != nil {
		t.Fatalf("CreateUser(first) error = %v", err)
	}
	if first.ID == 0 {
		t.Fatal("CreateUser(first) ID = 0, want generated ID")
	}
	if first.CreatedAt.IsZero() || first.UpdatedAt.IsZero() {
		t.Fatal("CreateUser(first) timestamps not populated")
	}

	count, err := db.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountUsers() = %d, want %d", count, 1)
	}

	loadedByEmail, err := db.GetUserByEmail(first.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if loadedByEmail.ID != first.ID {
		t.Fatalf("GetUserByEmail().ID = %d, want %d", loadedByEmail.ID, first.ID)
	}

	loadedByID, err := db.GetUserByID(first.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if loadedByID.Email != first.Email {
		t.Fatalf("GetUserByID().Email = %q, want %q", loadedByID.Email, first.Email)
	}

	first.Name = "updated"
	first.Role = "viewer"
	first.PasswordHash = "hash-2"
	if err := db.UpdateUser(first); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if first.Name != "updated" {
		t.Fatalf("UpdateUser().Name = %q, want %q", first.Name, "updated")
	}
	if first.Role != "viewer" {
		t.Fatalf("UpdateUser().Role = %q, want %q", first.Role, "viewer")
	}

	second := &model.User{
		Email:        "second@example.com",
		Name:         "second",
		PasswordHash: "hash-3",
		Role:         "viewer",
	}
	if err := db.CreateUser(second); err != nil {
		t.Fatalf("CreateUser(second) error = %v", err)
	}

	users, err := db.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers() len = %d, want %d", len(users), 2)
	}
	if users[0].ID != first.ID || users[1].ID != second.ID {
		t.Fatalf("ListUsers() order = [%d %d], want [%d %d]", users[0].ID, users[1].ID, first.ID, second.ID)
	}

	if err := db.DeleteUser(first.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	count, err = db.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers() after delete error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountUsers() after delete = %d, want %d", count, 1)
	}

	if _, err := db.GetUserByID(first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetUserByID() error = %v, want sql.ErrNoRows", err)
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}
