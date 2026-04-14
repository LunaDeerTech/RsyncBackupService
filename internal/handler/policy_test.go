package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
)

func TestPolicyCRUDListAndTrigger(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	createResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"instance_id":        instanceID,
		"name":               "nightly-cold",
		"type":               "cold",
		"target_id":          target.ID,
		"schedule_type":      "cron",
		"schedule_value":     "0 1 * * *",
		"bandwidth_limit_kb": 2048,
		"enabled":            true,
		"compression":        true,
		"encryption":         true,
		"encryption_key":     "SecretKey#1",
		"split_enabled":      true,
		"split_size_mb":      128,
		"retention_type":     "count",
		"retention_value":    7,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/instances/{id}/policies status = %d, want %d, body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	policy, err := db.GetPolicyByID(1)
	if err != nil {
		t.Fatalf("GetPolicyByID() error = %v", err)
	}
	if policy.EncryptionKeyHash == nil || !crypto.ValidateEncryptionKey("SecretKey#1", *policy.EncryptionKeyHash) {
		t.Fatalf("policy.EncryptionKeyHash = %v, want hashed SecretKey#1", policy.EncryptionKeyHash)
	}
	if policy.BandwidthLimitKB != 2048 {
		t.Fatalf("policy.BandwidthLimitKB = %d, want 2048", policy.BandwidthLimitKB)
	}
	if strings.Contains(createResponse.Body.String(), "SecretKey#1") {
		t.Fatal("create response leaked plaintext encryption key")
	}

	listResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/policies", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances/{id}/policies status = %d, want %d, body = %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	var listEnvelope apiEnvelope
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	var listPayload struct {
		Items []policyResponse `json:"items"`
	}
	if err := json.Unmarshal(listEnvelope.Data, &listPayload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != policy.ID {
		t.Fatalf("list payload = %+v, want one policy %d", listPayload, policy.ID)
	}
	if listPayload.Items[0].BandwidthLimitKB != 2048 {
		t.Fatalf("list bandwidth_limit_kb = %d, want 2048", listPayload.Items[0].BandwidthLimitKB)
	}
	if listPayload.Items[0].LastExecutionStatus != nil {
		t.Fatalf("initial LastExecutionStatus = %v, want nil", listPayload.Items[0].LastExecutionStatus)
	}

	triggerResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies/"+itoa(policy.ID)+"/trigger", map[string]any{"encryption_key": "SecretKey#1"}, mustAccessTokenForUser(t, admin, "secret"))
	if triggerResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/instances/{id}/policies/{pid}/trigger status = %d, want %d, body = %s", triggerResponse.Code, http.StatusCreated, triggerResponse.Body.String())
	}
	var triggerEnvelope apiEnvelope
	if err := json.Unmarshal(triggerResponse.Body.Bytes(), &triggerEnvelope); err != nil {
		t.Fatalf("decode trigger response: %v", err)
	}
	var triggerPayload struct {
		Backup model.Backup `json:"backup"`
		Task   model.Task   `json:"task"`
	}
	if err := json.Unmarshal(triggerEnvelope.Data, &triggerPayload); err != nil {
		t.Fatalf("decode trigger payload: %v", err)
	}
	if triggerPayload.Backup.Status != "pending" || triggerPayload.Task.Status != "queued" {
		t.Fatalf("trigger payload = %+v, want pending backup and queued task", triggerPayload)
	}

	listAfterTrigger := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/policies", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if listAfterTrigger.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances/{id}/policies after trigger status = %d, want %d", listAfterTrigger.Code, http.StatusOK)
	}
	if err := json.Unmarshal(listAfterTrigger.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list after trigger response: %v", err)
	}
	if err := json.Unmarshal(listEnvelope.Data, &listPayload); err != nil {
		t.Fatalf("decode list after trigger payload: %v", err)
	}
	if listPayload.Items[0].LatestBackupID == nil || *listPayload.Items[0].LatestBackupID != triggerPayload.Backup.ID {
		t.Fatalf("LatestBackupID = %v, want %d", listPayload.Items[0].LatestBackupID, triggerPayload.Backup.ID)
	}
	if listPayload.Items[0].LastExecutionStatus == nil || *listPayload.Items[0].LastExecutionStatus != "pending" {
		t.Fatalf("LastExecutionStatus = %v, want pending", listPayload.Items[0].LastExecutionStatus)
	}

	updateResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(instanceID)+"/policies/"+itoa(policy.ID), map[string]any{
		"name":               "nightly-cold-v2",
		"type":               "cold",
		"target_id":          target.ID,
		"schedule_type":      "interval",
		"schedule_value":     "7200",
		"bandwidth_limit_kb": -1,
		"enabled":            true,
		"compression":        false,
		"encryption":         true,
		"encryption_key":     "SecretKey#2",
		"split_enabled":      false,
		"retention_type":     "time",
		"retention_value":    14,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/instances/{id}/policies/{pid} status = %d, want %d, body = %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}

	updated, err := db.GetPolicyByID(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicyByID(updated) error = %v", err)
	}
	if updated.Name != "nightly-cold-v2" || updated.ScheduleType != "interval" || updated.BandwidthLimitKB != -1 {
		t.Fatalf("updated policy = %+v, want renamed interval policy", updated)
	}
	if updated.EncryptionKeyHash == nil || !crypto.ValidateEncryptionKey("SecretKey#2", *updated.EncryptionKeyHash) {
		t.Fatalf("updated EncryptionKeyHash = %v, want hashed SecretKey#2", updated.EncryptionKeyHash)
	}

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/instances/"+itoa(instanceID)+"/policies/"+itoa(policy.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/instances/{id}/policies/{pid} status = %d, want %d, body = %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	if _, err := db.GetPolicyByID(policy.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetPolicyByID(deleted) error = %v, want sql.ErrNoRows", err)
	}
	assertRowCount(t, db, `SELECT COUNT(*) FROM backups WHERE policy_id = ?`, policy.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM tasks WHERE backup_id = ?`, triggerPayload.Backup.ID, 0)
}

func TestPolicyValidationAndEncryptionUpdateBehavior(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"))

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	rollingTarget := &model.BackupTarget{
		Name:          "rolling-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(rollingTarget); err != nil {
		t.Fatalf("CreateBackupTarget(rolling) error = %v", err)
	}
	coldTarget := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(coldTarget); err != nil {
		t.Fatalf("CreateBackupTarget(cold) error = %v", err)
	}

	mismatch := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "bad-policy",
		"type":            "rolling",
		"target_id":       coldTarget.ID,
		"schedule_type":   "cron",
		"schedule_value":  "0 0 * * *",
		"enabled":         true,
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, mismatch, http.StatusBadRequest, authErrorInvalidRequest, "policy type rolling is incompatible with target backup_type cold")

	invalidCron := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "bad-cron",
		"type":            "rolling",
		"target_id":       rollingTarget.ID,
		"schedule_type":   "cron",
		"schedule_value":  "invalid cron",
		"enabled":         true,
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, invalidCron, http.StatusBadRequest, authErrorInvalidRequest, "schedule_value must be a standard 5-field cron expression")

	invalidBandwidthLimit := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":               "bad-limit",
		"type":               "rolling",
		"target_id":          rollingTarget.ID,
		"schedule_type":      "interval",
		"schedule_value":     "60",
		"bandwidth_limit_kb": 0,
		"enabled":            true,
		"retention_type":     "count",
		"retention_value":    3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, invalidBandwidthLimit, http.StatusBadRequest, authErrorInvalidRequest, "bandwidth_limit_kb must be -1 or a positive integer")

	rollingWithColdOptions := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "rolling-cold-options",
		"type":            "rolling",
		"target_id":       rollingTarget.ID,
		"schedule_type":   "interval",
		"schedule_value":  "60",
		"enabled":         true,
		"compression":     true,
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, rollingWithColdOptions, http.StatusBadRequest, authErrorInvalidRequest, "compression is only supported for cold policies")

	missingKey := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "missing-key",
		"type":            "cold",
		"target_id":       coldTarget.ID,
		"schedule_type":   "interval",
		"schedule_value":  "60",
		"enabled":         true,
		"encryption":      true,
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, missingKey, http.StatusBadRequest, authErrorInvalidRequest, "encryption_key is required when encryption is enabled")

	missingSplitSize := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "missing-split-size",
		"type":            "cold",
		"target_id":       coldTarget.ID,
		"schedule_type":   "interval",
		"schedule_value":  "60",
		"enabled":         true,
		"split_enabled":   true,
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, missingSplitSize, http.StatusBadRequest, authErrorInvalidRequest, "split_size_mb must be positive when split_enabled is true")

	createGood := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/policies", map[string]any{
		"name":            "keep-key",
		"type":            "cold",
		"target_id":       coldTarget.ID,
		"schedule_type":   "interval",
		"schedule_value":  "60",
		"enabled":         true,
		"encryption":      true,
		"encryption_key":  "Persisted#1",
		"retention_type":  "count",
		"retention_value": 3,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createGood.Code != http.StatusCreated {
		t.Fatalf("create valid policy status = %d, want %d, body = %s", createGood.Code, http.StatusCreated, createGood.Body.String())
	}
	policy, err := db.GetPolicyByID(1)
	if err != nil {
		t.Fatalf("GetPolicyByID(valid) error = %v", err)
	}
	if policy.BandwidthLimitKB != -1 {
		t.Fatalf("default BandwidthLimitKB = %d, want -1", policy.BandwidthLimitKB)
	}
	originalHash := *policy.EncryptionKeyHash

	updateKeepKey := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(instanceID)+"/policies/"+itoa(policy.ID), map[string]any{
		"name":            "keep-key-updated",
		"type":            "cold",
		"target_id":       coldTarget.ID,
		"schedule_type":   "interval",
		"schedule_value":  "120",
		"enabled":         true,
		"encryption":      true,
		"retention_type":  "count",
		"retention_value": 5,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateKeepKey.Code != http.StatusOK {
		t.Fatalf("update without new encryption_key status = %d, want %d, body = %s", updateKeepKey.Code, http.StatusOK, updateKeepKey.Body.String())
	}
	updated, err := db.GetPolicyByID(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicyByID(updated) error = %v", err)
	}
	if updated.EncryptionKeyHash == nil || *updated.EncryptionKeyHash != originalHash {
		t.Fatalf("updated.EncryptionKeyHash = %v, want preserved hash %q", updated.EncryptionKeyHash, originalHash)
	}
}
