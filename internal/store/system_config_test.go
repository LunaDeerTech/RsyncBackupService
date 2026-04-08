package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestSystemConfigCRUD(t *testing.T) {
	db := newStoreTestDB(t)

	if err := db.SetSystemConfig("registration.enabled", "false"); err != nil {
		t.Fatalf("SetSystemConfig() error = %v", err)
	}
	value, err := db.GetSystemConfig("registration.enabled")
	if err != nil {
		t.Fatalf("GetSystemConfig() error = %v", err)
	}
	if value != "false" {
		t.Fatalf("GetSystemConfig() = %q, want %q", value, "false")
	}

	values, err := db.GetSystemConfigs([]string{"registration.enabled", "smtp.host", "missing"})
	if err != nil {
		t.Fatalf("GetSystemConfigs() error = %v", err)
	}
	if values["registration.enabled"] != "false" {
		t.Fatalf("registration.enabled = %q, want %q", values["registration.enabled"], "false")
	}
	if _, ok := values["missing"]; ok {
		t.Fatal("missing config unexpectedly returned")
	}
}

func TestNotificationSubscriptionQueries(t *testing.T) {
	db := newStoreTestDB(t)
	user := createSystemConfigTestUser(t, db, "viewer@example.com", "viewer")
	instanceA := createSystemConfigTestInstance(t, db, "instance-a")
	instanceB := createSystemConfigTestInstance(t, db, "instance-b")

	if err := db.UpdateSubscriptions(user.ID, []model.NotificationSubscription{{InstanceID: instanceB.ID, Enabled: true}, {InstanceID: instanceA.ID, Enabled: true}}); err != nil {
		t.Fatalf("UpdateSubscriptions() error = %v", err)
	}

	subscriptions, err := db.ListSubscriptionsByUser(user.ID)
	if err != nil {
		t.Fatalf("ListSubscriptionsByUser() error = %v", err)
	}
	if len(subscriptions) != 2 {
		t.Fatalf("subscriptions len = %d, want %d", len(subscriptions), 2)
	}
	if subscriptions[0].InstanceID != instanceA.ID || subscriptions[0].InstanceName != instanceA.Name {
		t.Fatalf("first subscription = %+v, want instance-a", subscriptions[0])
	}

	subscribers, err := db.ListSubscribersByInstance(instanceA.ID)
	if err != nil {
		t.Fatalf("ListSubscribersByInstance() error = %v", err)
	}
	if len(subscribers) != 1 || subscribers[0].ID != user.ID {
		t.Fatalf("subscribers = %+v, want viewer", subscribers)
	}

	if err := db.UpdateSubscriptions(user.ID, []model.NotificationSubscription{{InstanceID: instanceA.ID, Enabled: false}}); err != nil {
		t.Fatalf("UpdateSubscriptions(disable) error = %v", err)
	}
	subscribers, err = db.ListSubscribersByInstance(instanceA.ID)
	if err != nil {
		t.Fatalf("ListSubscribersByInstance(disabled) error = %v", err)
	}
	if len(subscribers) != 0 {
		t.Fatalf("disabled subscribers len = %d, want 0", len(subscribers))
	}
}

func newStoreTestDB(t *testing.T) *DB {
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

func createSystemConfigTestUser(t *testing.T, db *DB, email, role string) *model.User {
	t.Helper()
	user := &model.User{Email: email, Name: email, PasswordHash: "hash", Role: role}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(%q) error = %v", email, err)
	}
	return user
}

func createSystemConfigTestInstance(t *testing.T, db *DB, name string) *model.Instance {
	t.Helper()
	instance := &model.Instance{Name: name, SourceType: "local", SourcePath: "/data/" + name, Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance(%q) error = %v", name, err)
	}
	return instance
}

func TestGetSystemConfigMissingReturnsNoRows(t *testing.T) {
	db := newStoreTestDB(t)
	_, err := db.GetSystemConfig("missing")
	if !errors.Is(err, sql.ErrNoRows) && (err == nil || !strings.Contains(err.Error(), sql.ErrNoRows.Error())) {
		t.Fatalf("GetSystemConfig(missing) error = %v, want sql.ErrNoRows", err)
	}
}