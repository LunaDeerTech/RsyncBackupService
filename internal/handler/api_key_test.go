package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
	servicepkg "rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestCurrentUserAPIKeyLifecycleAndV2Access(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	instanceID := createHandlerTestInstance(t, db, "instance-one")
	otherInstanceID := createHandlerTestInstance(t, db, "instance-two")
	targetID := createHandlerTestBackupTarget(t, db, "target-one")
	policyID := createHandlerTestPolicy(t, db, instanceID, targetID, "daily")
	backupID := insertHandlerTestBackup(t, db, instanceID, policyID, "success", 128, 96, "CURRENT_TIMESTAMP")
	insertHandlerTestTaskWithStatus(t, db, instanceID, backupID, "running")
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readonly"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	fixedNow := time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC)
	scheduler := engine.NewScheduler(db, nil)
	scheduler.SetClock(func() time.Time { return fixedNow })
	disasterRecovery := servicepkg.NewDisasterRecoveryService(db)
	disasterRecovery.SetClock(func() time.Time { return fixedNow })
	router := NewRouter(db, WithJWTSecret("secret"), WithScheduler(scheduler), WithDisasterRecoveryService(disasterRecovery))
	viewerToken := mustAccessTokenForUser(t, viewer, "secret")

	createResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/users/me/api-keys", map[string]string{"name": "default"}, viewerToken)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/users/me/api-keys status = %d, want %d, body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("decode create api key response: %v", err)
	}
	var created apiKeyCreateResponse
	if err := json.Unmarshal(createEnvelope.Data, &created); err != nil {
		t.Fatalf("decode create api key payload: %v", err)
	}
	if created.APIKey.Name != "default" || created.Key == "" {
		t.Fatalf("created api key payload = %+v, want named key with raw token", created)
	}
	if created.APIKey.KeyPrefix == created.Key {
		t.Fatal("created api key prefix unexpectedly equals full key")
	}

	listResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/users/me/api-keys", nil, viewerToken)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users/me/api-keys status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var listEnvelope apiEnvelope
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list api key response: %v", err)
	}
	var listPayload struct {
		Items []apiKeyResponse `json:"items"`
	}
	if err := json.Unmarshal(listEnvelope.Data, &listPayload); err != nil {
		t.Fatalf("decode list api key payload: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != created.APIKey.ID {
		t.Fatalf("list api key payload = %+v, want created key", listPayload)
	}

	instancesResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances?page=1&page_size=20", nil, created.Key)
	if instancesResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances status = %d, want %d, body = %s", instancesResponse.Code, http.StatusOK, instancesResponse.Body.String())
	}
	var instancesEnvelope apiEnvelope
	if err := json.Unmarshal(instancesResponse.Body.Bytes(), &instancesEnvelope); err != nil {
		t.Fatalf("decode v2 instances response: %v", err)
	}
	var instancesPage struct {
		Items []instanceListItem `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(instancesEnvelope.Data, &instancesPage); err != nil {
		t.Fatalf("decode v2 instances payload: %v", err)
	}
	if instancesPage.Total != 1 || len(instancesPage.Items) != 1 || instancesPage.Items[0].ID != instanceID {
		t.Fatalf("v2 instances payload = %+v, want only authorized instance %d", instancesPage, instanceID)
	}
	assertJSONKeyAbsent(t, instancesEnvelope.Data, "items.0.source_path")
	assertJSONKeyAbsent(t, instancesEnvelope.Data, "items.0.source_type")

	overviewResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/overview", nil, created.Key)
	if overviewResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/overview status = %d, want %d, body = %s", overviewResponse.Code, http.StatusOK, overviewResponse.Body.String())
	}
	var overviewEnvelope apiEnvelope
	if err := json.Unmarshal(overviewResponse.Body.Bytes(), &overviewEnvelope); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	var overview instanceDetailResponse
	if err := json.Unmarshal(overviewEnvelope.Data, &overview); err != nil {
		t.Fatalf("decode overview payload: %v", err)
	}
	if overview.Instance.ID != instanceID || overview.Stats.BackupCount != 1 || overview.Permission != "readonly" {
		t.Fatalf("overview payload = %+v, want authorized instance overview", overview)
	}
	assertJSONKeyAbsent(t, overviewEnvelope.Data, "instance.source_path")
	assertJSONKeyAbsent(t, overviewEnvelope.Data, "instance.source_type")
	assertJSONKeyAbsent(t, overviewEnvelope.Data, "stats.last_backup.snapshot_path")
	assertJSONKeyAbsent(t, overviewEnvelope.Data, "stats.last_backup.rsync_stats")
	assertJSONKeyAbsent(t, overviewEnvelope.Data, "stats.last_backup.trigger_source")

	currentTaskResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/current-task", nil, created.Key)
	if currentTaskResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/current-task status = %d, want %d, body = %s", currentTaskResponse.Code, http.StatusOK, currentTaskResponse.Body.String())
	}
	var currentTaskEnvelope apiEnvelope
	if err := json.Unmarshal(currentTaskResponse.Body.Bytes(), &currentTaskEnvelope); err != nil {
		t.Fatalf("decode current task response: %v", err)
	}
	var currentTaskPayload struct {
		Task *taskResponse `json:"task"`
	}
	if err := json.Unmarshal(currentTaskEnvelope.Data, &currentTaskPayload); err != nil {
		t.Fatalf("decode current task payload: %v", err)
	}
	if currentTaskPayload.Task == nil || currentTaskPayload.Task.Status != "running" {
		t.Fatalf("current task payload = %+v, want running task", currentTaskPayload)
	}

	policiesResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/policies", nil, created.Key)
	if policiesResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/policies status = %d, want %d, body = %s", policiesResponse.Code, http.StatusOK, policiesResponse.Body.String())
	}
	var policiesEnvelope apiEnvelope
	if err := json.Unmarshal(policiesResponse.Body.Bytes(), &policiesEnvelope); err != nil {
		t.Fatalf("decode policies response: %v", err)
	}
	var policiesPayload struct {
		Items []policyResponse `json:"items"`
	}
	if err := json.Unmarshal(policiesEnvelope.Data, &policiesPayload); err != nil {
		t.Fatalf("decode policies payload: %v", err)
	}
	if len(policiesPayload.Items) != 1 || policiesPayload.Items[0].ID != policyID || policiesPayload.Items[0].InstanceID != instanceID {
		t.Fatalf("policies payload = %+v, want one policy %d", policiesPayload, policyID)
	}

	planResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/plan?within_hours=24", nil, created.Key)
	if planResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/plan status = %d, want %d, body = %s", planResponse.Code, http.StatusOK, planResponse.Body.String())
	}
	var planEnvelope apiEnvelope
	if err := json.Unmarshal(planResponse.Body.Bytes(), &planEnvelope); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}
	var planPayload struct {
		Items []engine.UpcomingTask `json:"items"`
	}
	if err := json.Unmarshal(planEnvelope.Data, &planPayload); err != nil {
		t.Fatalf("decode plan payload: %v", err)
	}
	if len(planPayload.Items) != 1 || planPayload.Items[0].InstanceID != instanceID || planPayload.Items[0].PolicyID != policyID {
		t.Fatalf("plan payload = %+v, want upcoming task for policy %d", planPayload, policyID)
	}

	disasterRecoveryResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/disaster-recovery", nil, created.Key)
	if disasterRecoveryResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/disaster-recovery status = %d, want %d, body = %s", disasterRecoveryResponse.Code, http.StatusOK, disasterRecoveryResponse.Body.String())
	}
	var disasterRecoveryEnvelope apiEnvelope
	if err := json.Unmarshal(disasterRecoveryResponse.Body.Bytes(), &disasterRecoveryEnvelope); err != nil {
		t.Fatalf("decode disaster recovery response: %v", err)
	}
	var disasterRecoveryPayload servicepkg.DisasterRecoveryScore
	if err := json.Unmarshal(disasterRecoveryEnvelope.Data, &disasterRecoveryPayload); err != nil {
		t.Fatalf("decode disaster recovery payload: %v", err)
	}
	if disasterRecoveryPayload.Level == "" || disasterRecoveryPayload.Total <= 0 {
		t.Fatalf("disaster recovery payload = %+v, want populated score", disasterRecoveryPayload)
	}

	backupsResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(instanceID)+"/backups?page=1&page_size=20", nil, created.Key)
	if backupsResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/instances/{id}/backups status = %d, want %d, body = %s", backupsResponse.Code, http.StatusOK, backupsResponse.Body.String())
	}
	var backupsEnvelope apiEnvelope
	if err := json.Unmarshal(backupsResponse.Body.Bytes(), &backupsEnvelope); err != nil {
		t.Fatalf("decode backups response: %v", err)
	}
	var backupsPage struct {
		Items []v2BackupResponse `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(backupsEnvelope.Data, &backupsPage); err != nil {
		t.Fatalf("decode backups payload: %v", err)
	}
	if backupsPage.Total != 1 || len(backupsPage.Items) != 1 || backupsPage.Items[0].ID != backupID {
		t.Fatalf("backups payload = %+v, want one backup %d", backupsPage, backupID)
	}
	assertJSONKeyAbsent(t, backupsEnvelope.Data, "items.0.snapshot_path")
	assertJSONKeyAbsent(t, backupsEnvelope.Data, "items.0.rsync_stats")
	assertJSONKeyAbsent(t, backupsEnvelope.Data, "items.0.trigger_source")

	forbiddenResponse := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances/"+itoa(otherInstanceID)+"/overview", nil, created.Key)
	assertAPIError(t, forbiddenResponse, http.StatusForbidden, 40301, "forbidden")

	adminKey := mustCreateHandlerAPIKey(t, db, admin.ID, "admin-key")
	adminInstances := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances", nil, adminKey)
	if adminInstances.Code != http.StatusOK {
		t.Fatalf("admin GET /api/v2/instances status = %d, want %d", adminInstances.Code, http.StatusOK)
	}

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/users/me/api-keys/"+itoa(created.APIKey.ID), nil, viewerToken)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/users/me/api-keys/{id} status = %d, want %d", deleteResponse.Code, http.StatusOK)
	}

	unauthorizedAfterDelete := performAPIKeyJSONRequest(t, router, http.MethodGet, "/api/v2/instances", nil, created.Key)
	assertAPIError(t, unauthorizedAfterDelete, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
}

func performAPIKeyJSONRequest(t *testing.T, handler http.Handler, method, path string, body any, apiKey string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	request := httptest.NewRequest(method, path, requestBody)
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	return recorder
}

func mustCreateHandlerAPIKey(t *testing.T, db handlerAPIKeyStore, userID int64, name string) string {
	t.Helper()

	rawKey, err := authcrypto.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	hash, err := authcrypto.HashAPIKey(rawKey)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}
	apiKey := &model.APIKey{
		UserID:    userID,
		Name:      name,
		KeyPrefix: authcrypto.APIKeyDisplayPrefix(rawKey),
		KeyHash:   hash,
	}
	if err := db.CreateAPIKey(apiKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	return rawKey
}

func createHandlerTestBackupTarget(t *testing.T, db *store.DB, name string) int64 {
	t.Helper()

	target := &model.BackupTarget{
		Name:          name,
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget(%q) error = %v", name, err)
	}

	return target.ID
}

func insertHandlerTestTaskWithStatus(t *testing.T, db *store.DB, instanceID, backupID int64, status string) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO tasks (instance_id, backup_id, type, status, progress, current_step, started_at, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)`,
		instanceID,
		backupID,
		"backup",
		status,
		35,
		"transferring data",
		"",
	); err != nil {
		t.Fatalf("insert task error = %v", err)
	}
}

type handlerAPIKeyStore interface {
	CreateAPIKey(apiKey *model.APIKey) error
}

func assertJSONKeyAbsent(t *testing.T, payload []byte, path string) {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload for %s: %v", path, err)
	}

	current := decoded
	for _, segment := range bytes.Split([]byte(path), []byte(".")) {
		key := string(segment)
		switch value := current.(type) {
		case map[string]any:
			next, ok := value[key]
			if !ok {
				return
			}
			current = next
		case []any:
			position, err := strconv.Atoi(key)
			if err != nil || position < 0 || position >= len(value) {
				return
			}
			current = value[position]
		default:
			return
		}
	}

	t.Fatalf("payload unexpectedly contains %s", path)
}
