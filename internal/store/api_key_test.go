package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
)

func TestAPIKeyCRUD(t *testing.T) {
	db := newTestDB(t)
	user := &model.User{
		Email:        "apikey@example.com",
		Name:         "API Key User",
		PasswordHash: "hash",
		Role:         "viewer",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	apiKey := &model.APIKey{
		UserID:    user.ID,
		Name:      "default",
		KeyPrefix: "rbs_12345678",
		KeyHash:   "hash-value",
		Key:       "rbs_secret",
	}
	if err := db.CreateAPIKey(apiKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if apiKey.ID == 0 {
		t.Fatal("CreateAPIKey() ID = 0, want generated ID")
	}
	if apiKey.CreatedAt.IsZero() {
		t.Fatal("CreateAPIKey() CreatedAt is zero")
	}
	if apiKey.Key != "rbs_secret" {
		t.Fatalf("CreateAPIKey() Key = %q, want %q", apiKey.Key, "rbs_secret")
	}

	loadedByHash, err := db.GetAPIKeyByHash("hash-value")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash() error = %v", err)
	}
	if loadedByHash.ID != apiKey.ID || loadedByHash.Name != apiKey.Name {
		t.Fatalf("loadedByHash = %+v, want created api key", loadedByHash)
	}

	items, err := db.ListAPIKeysByUser(user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeysByUser() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != apiKey.ID {
		t.Fatalf("ListAPIKeysByUser() = %+v, want one api key %d", items, apiKey.ID)
	}
	if items[0].LastUsedAt != nil {
		t.Fatalf("initial LastUsedAt = %v, want nil", items[0].LastUsedAt)
	}

	if err := db.TouchAPIKeyLastUsed(apiKey.ID); err != nil {
		t.Fatalf("TouchAPIKeyLastUsed() error = %v", err)
	}

	loadedByID, err := db.GetAPIKeyByID(apiKey.ID)
	if err != nil {
		t.Fatalf("GetAPIKeyByID() error = %v", err)
	}
	if loadedByID.LastUsedAt == nil {
		t.Fatal("GetAPIKeyByID().LastUsedAt = nil, want populated timestamp")
	}
	if time.Since(*loadedByID.LastUsedAt) > time.Minute {
		t.Fatalf("GetAPIKeyByID().LastUsedAt = %v, want recent timestamp", *loadedByID.LastUsedAt)
	}

	if err := db.DeleteAPIKeyByIDAndUser(apiKey.ID, user.ID); err != nil {
		t.Fatalf("DeleteAPIKeyByIDAndUser() error = %v", err)
	}
	if _, err := db.GetAPIKeyByID(apiKey.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetAPIKeyByID(deleted) error = %v, want sql.ErrNoRows", err)
	}
}
