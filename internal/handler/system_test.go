package handler

import (
	"encoding/json"
	"net/http"
	"net/smtp"
	"strings"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/service"
)

func TestSMTPSystemEndpoints(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	systemConfigs := service.NewSystemConfigService(db, authcrypto.DeriveAESKey("secret"))
	called := false
	systemConfigs.SetMailer(func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		called = true
		if addr != "smtp.example.com:587" || from != "noreply@example.com" || len(to) != 1 || to[0] != "test@example.com" {
			t.Fatalf("mailer args = %q %q %+v", addr, from, to)
		}
		if !strings.Contains(string(msg), "SMTP 测试邮件") {
			t.Fatalf("message = %q, want smtp test message", msg)
		}
		return nil
	})
	router := NewRouter(db, WithJWTSecret("secret"), WithSystemConfigService(systemConfigs))
	token := mustAccessTokenForUser(t, admin, "secret")

	update := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/system/smtp", map[string]any{
		"host":     "smtp.example.com",
		"port":     587,
		"username": "mailer",
		"password": "smtp-pass",
		"from":     "noreply@example.com",
	}, token)
	if update.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/system/smtp status = %d, want %d", update.Code, http.StatusOK)
	}

	get := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/system/smtp", nil, token)
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/smtp status = %d, want %d", get.Code, http.StatusOK)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(get.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode get smtp response: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode smtp payload: %v", err)
	}
	if payload["password"] != "***" {
		t.Fatalf("masked password = %+v, want ***", payload["password"])
	}

	testResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/system/smtp/test", map[string]string{"to": "test@example.com"}, token)
	if testResponse.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/system/smtp/test status = %d, want %d", testResponse.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("smtp test endpoint did not call configured mailer")
	}
}

func TestCurrentUserSubscriptionsEndpoints(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	instanceA := createHandlerTestInstance(t, db, "instance-a")
	instanceB := createHandlerTestInstance(t, db, "instance-b")
	if _, err := db.Exec(`INSERT INTO instance_permissions (user_id, instance_id, permission, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`, viewer.ID, instanceA, "readonly"); err != nil {
		t.Fatalf("insert instance permission error = %v", err)
	}
	router := NewRouter(db, WithJWTSecret("secret"))
	token := mustAccessTokenForUser(t, viewer, "secret")

	get := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/users/me/subscriptions", nil, token)
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users/me/subscriptions status = %d, want %d", get.Code, http.StatusOK)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(get.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode get subscriptions response: %v", err)
	}
	var data struct {
		Subscriptions []map[string]any `json:"subscriptions"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode subscriptions payload: %v", err)
	}
	if len(data.Subscriptions) != 1 || int64(data.Subscriptions[0]["instance_id"].(float64)) != instanceA {
		t.Fatalf("subscriptions payload = %+v, want only instance-a", data.Subscriptions)
	}
	if enabled, _ := data.Subscriptions[0]["enabled"].(bool); enabled {
		t.Fatal("default subscription enabled = true, want false")
	}

	update := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/me/subscriptions", map[string]any{
		"subscriptions": []map[string]any{{"instance_id": instanceA, "enabled": true}},
	}, token)
	if update.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/users/me/subscriptions status = %d, want %d", update.Code, http.StatusOK)
	}
	subscribers, err := db.ListSubscribersByInstance(instanceA)
	if err != nil {
		t.Fatalf("ListSubscribersByInstance() error = %v", err)
	}
	if len(subscribers) != 1 || subscribers[0].ID != viewer.ID {
		t.Fatalf("subscribers = %+v, want viewer", subscribers)
	}

	invalid := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/me/subscriptions", map[string]any{
		"subscriptions": []map[string]any{{"instance_id": instanceB, "enabled": true}},
	}, token)
	assertAPIError(t, invalid, http.StatusBadRequest, authErrorInvalidRequest, "subscription instance is not accessible")
}