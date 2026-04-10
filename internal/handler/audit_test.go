package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestAuditLogAPIsSupportFiltersAndInstancePermissions(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	instanceA := createHandlerTestInstance(t, db, "alpha")
	instanceB := createHandlerTestInstance(t, db, "beta")
	if err := db.SetInstancePermissions(instanceA, []model.InstancePermission{{UserID: viewer.ID, InstanceID: instanceA, Permission: "readonly"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}
	insertHandlerAuditRow(t, db, &instanceA, &admin.ID, "instance.create", `{"name":"alpha"}`, time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC))
	insertHandlerAuditRow(t, db, &instanceA, &viewer.ID, "backup.trigger", `{"backup_id":11}`, time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC))
	insertHandlerAuditRow(t, db, &instanceB, &admin.ID, "instance.update", `{"name":"beta"}`, time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC))

	router := NewRouter(db, WithJWTSecret("secret"))
	globalResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/audit-logs?action=backup.trigger,instance.update&start_date=2026-04-07&end_date=2026-04-07&page=1&page_size=10", nil, mustAccessTokenForUser(t, admin, "secret"))
	if globalResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/audit-logs status = %d, want %d, body = %s", globalResponse.Code, http.StatusOK, globalResponse.Body.String())
	}
	globalPayload := decodeAuditListResponse(t, globalResponse)
	if globalPayload.Total != 2 {
		t.Fatalf("global payload total = %d, want %d", globalPayload.Total, 2)
	}
	if len(globalPayload.Items) != 2 {
		t.Fatalf("len(global payload items) = %d, want %d", len(globalPayload.Items), 2)
	}
	if globalPayload.Items[0].Action != "instance.update" || globalPayload.Items[0].UserName != "Admin" {
		t.Fatalf("global first item = %+v, want latest admin instance.update", globalPayload.Items[0])
	}

	instanceResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceA)+"/audit-logs?action=backup.trigger&page=1&page_size=10", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if instanceResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances/{id}/audit-logs status = %d, want %d, body = %s", instanceResponse.Code, http.StatusOK, instanceResponse.Body.String())
	}
	instancePayload := decodeAuditListResponse(t, instanceResponse)
	if instancePayload.Total != 1 || len(instancePayload.Items) != 1 {
		t.Fatalf("instance payload = %+v, want single backup.trigger entry", instancePayload)
	}
	if instancePayload.Items[0].Action != "backup.trigger" {
		t.Fatalf("instance item action = %q, want %q", instancePayload.Items[0].Action, "backup.trigger")
	}

	forbidden := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceB)+"/audit-logs", nil, mustAccessTokenForUser(t, viewer, "secret"))
	assertAPIError(t, forbidden, http.StatusForbidden, 40301, "forbidden")
}

func TestInstanceHandlersWriteAuditLogs(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"))

	remote := &model.RemoteConfig{
		Name:           "ssh-remote",
		Type:           "ssh",
		Host:           "10.0.0.5",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: filepath.Join(t.TempDir(), "id_rsa"),
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	createResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances", map[string]any{
		"name":             "mysql-prod",
		"source_type":      "ssh",
		"source_path":      "/srv/mysql",
		"exclude_patterns": []string{"*.log"},
		"remote_config_id": remote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/instances status = %d, want %d, body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	instance, err := db.GetInstanceByName("mysql-prod")
	if err != nil {
		t.Fatalf("GetInstanceByName() error = %v", err)
	}

	updateResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(instance.ID), map[string]any{
		"name":             "mysql-main",
		"source_type":      "ssh",
		"source_path":      "/data/mysql",
		"exclude_patterns": []string{"node_modules/"},
		"remote_config_id": remote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/instances/{id} status = %d, want %d, body = %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/instances/"+itoa(instance.ID), map[string]string{
		"instance_name": "mysql-main",
		"password":      "AdminPass123",
	}, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/instances/{id} status = %d, want %d, body = %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	assertAuditActionCount(t, db, "instance.create", 1)
	assertAuditActionCount(t, db, "instance.update", 1)
	assertAuditActionCount(t, db, "instance.delete", 1)

	var detail string
	if err := db.QueryRow(`SELECT detail FROM audit_logs WHERE action = ? ORDER BY id DESC LIMIT 1`, "instance.delete").Scan(&detail); err != nil {
		t.Fatalf("query instance.delete detail error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(detail), &payload); err != nil {
		t.Fatalf("json.Unmarshal(instance.delete detail) error = %v", err)
	}
	if int64(payload["deleted_instance_id"].(float64)) != instance.ID {
		t.Fatalf("deleted_instance_id = %v, want %d", payload["deleted_instance_id"], instance.ID)
	}
	if _, ok := payload["exclude_patterns"]; !ok {
		t.Fatalf("instance.delete detail missing exclude_patterns: %v", payload)
	}
}

type auditListPayload struct {
	Items      []model.AuditLogWithUser `json:"items"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	TotalPages int                      `json:"total_pages"`
}

func decodeAuditListResponse(t *testing.T, recorder *httptest.ResponseRecorder) auditListPayload {
	t.Helper()

	var envelope apiEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope error = %v", err)
	}
	var payload auditListPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload error = %v", err)
	}
	return payload
}

func insertHandlerAuditRow(t *testing.T, db *store.DB, instanceID, userID *int64, action, detail string, createdAt time.Time) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (?, ?, ?, ?, ?)`, instanceID, userID, action, detail, createdAt.Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("insert audit log error = %v", err)
	}
}

func assertAuditActionCount(t *testing.T, db *store.DB, action string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action = ?`, action).Scan(&got); err != nil {
		t.Fatalf("count audit logs for %q error = %v", action, err)
	}
	if got != want {
		t.Fatalf("audit logs for %q = %d, want %d", action, got, want)
	}
}
