package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestInstanceCRUDStatsAndPermissions(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
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
		"exclude_patterns": []string{"*.log", "tmp/**", "*.log"},
		"remote_config_id": remote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/instances status = %d, want %d, body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	instance, err := db.GetInstanceByName("mysql-prod")
	if err != nil {
		t.Fatalf("GetInstanceByName() error = %v", err)
	}

	otherInstance := &model.Instance{
		Name:       "postgres-prod",
		SourceType: "local",
		SourcePath: "/srv/postgres",
		Status:     "idle",
	}
	if err := db.CreateInstance(otherInstance); err != nil {
		t.Fatalf("CreateInstance(other) error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "primary-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policyID := createHandlerTestPolicy(t, db, instance.ID, target.ID, "daily-backup")
	firstBackupID := insertHandlerTestBackup(t, db, instance.ID, policyID, "failed", 150, 120, "datetime('now', '-1 day')")
	_ = firstBackupID
	secondBackupID := insertHandlerTestBackup(t, db, instance.ID, policyID, "success", 80, 60, "CURRENT_TIMESTAMP")
	insertHandlerTestTask(t, db, instance.ID, secondBackupID)
	insertHandlerTestNotificationSubscription(t, db, viewer.ID, instance.ID)
	insertHandlerTestAuditLog(t, db, admin.ID, instance.ID)

	permissionsResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(instance.ID)+"/permissions", map[string]any{
		"permissions": []map[string]any{{
			"user_id":    viewer.ID,
			"permission": "readonly",
		}},
	}, mustAccessTokenForUser(t, admin, "secret"))
	if permissionsResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/instances/{id}/permissions status = %d, want %d, body = %s", permissionsResponse.Code, http.StatusOK, permissionsResponse.Body.String())
	}

	adminList := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances?page=1&page_size=10", nil, mustAccessTokenForUser(t, admin, "secret"))
	if adminList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances admin status = %d, want %d, body = %s", adminList.Code, http.StatusOK, adminList.Body.String())
	}
	var adminEnvelope apiEnvelope
	if err := json.Unmarshal(adminList.Body.Bytes(), &adminEnvelope); err != nil {
		t.Fatalf("decode admin list response: %v", err)
	}
	var adminPage struct {
		Items []instanceListItem `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(adminEnvelope.Data, &adminPage); err != nil {
		t.Fatalf("decode admin list payload: %v", err)
	}
	if adminPage.Total != 2 || len(adminPage.Items) != 2 {
		t.Fatalf("admin list payload = %+v, want two instances", adminPage)
	}
	if adminPage.Items[0].BackupCount != 2 {
		t.Fatalf("admin list first BackupCount = %d, want %d", adminPage.Items[0].BackupCount, 2)
	}
	if adminPage.Items[0].LastBackupStatus == nil || *adminPage.Items[0].LastBackupStatus != "success" {
		t.Fatalf("admin list first LastBackupStatus = %v, want success", adminPage.Items[0].LastBackupStatus)
	}

	viewerList := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances?page=1&page_size=10", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if viewerList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances viewer status = %d, want %d, body = %s", viewerList.Code, http.StatusOK, viewerList.Body.String())
	}
	var viewerEnvelope apiEnvelope
	if err := json.Unmarshal(viewerList.Body.Bytes(), &viewerEnvelope); err != nil {
		t.Fatalf("decode viewer list response: %v", err)
	}
	var viewerPage struct {
		Items []instanceListItem `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(viewerEnvelope.Data, &viewerPage); err != nil {
		t.Fatalf("decode viewer list payload: %v", err)
	}
	if viewerPage.Total != 1 || len(viewerPage.Items) != 1 || viewerPage.Items[0].ID != instance.ID {
		t.Fatalf("viewer list payload = %+v, want only authorized instance %d", viewerPage, instance.ID)
	}

	detailResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instance.ID), nil, mustAccessTokenForUser(t, viewer, "secret"))
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances/{id} status = %d, want %d, body = %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	var detailEnvelope apiEnvelope
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailEnvelope); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	var detail instanceDetailResponse
	if err := json.Unmarshal(detailEnvelope.Data, &detail); err != nil {
		t.Fatalf("decode detail payload: %v", err)
	}
	if detail.Stats.BackupCount != 2 || detail.Stats.PolicyCount != 1 {
		t.Fatalf("detail stats = %+v, want backup_count=2 policy_count=1", detail.Stats)
	}
	if detail.Stats.LastBackup == nil || detail.Stats.LastBackup.ID != secondBackupID {
		t.Fatalf("detail stats last backup = %+v, want backup %d", detail.Stats.LastBackup, secondBackupID)
	}
	if len(detail.Instance.ExcludePatterns) != 2 || detail.Instance.ExcludePatterns[0] != "*.log" || detail.Instance.ExcludePatterns[1] != "tmp/**" {
		t.Fatalf("detail instance exclude_patterns = %#v, want normalized patterns", detail.Instance.ExcludePatterns)
	}

	statsResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instance.ID)+"/stats", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances/{id}/stats status = %d, want %d, body = %s", statsResponse.Code, http.StatusOK, statsResponse.Body.String())
	}
	var statsEnvelope apiEnvelope
	if err := json.Unmarshal(statsResponse.Body.Bytes(), &statsEnvelope); err != nil {
		t.Fatalf("decode stats response: %v", err)
	}
	var stats model.InstanceStats
	if err := json.Unmarshal(statsEnvelope.Data, &stats); err != nil {
		t.Fatalf("decode stats payload: %v", err)
	}
	if stats.BackupCount != 2 || stats.SuccessBackupCount != 1 || stats.FailureBackupCount != 1 {
		t.Fatalf("stats payload = %+v, want backup_count=2 success=1 failure=1", stats)
	}
	if len(stats.RecentTrend) != 7 {
		t.Fatalf("len(stats.RecentTrend) = %d, want %d", len(stats.RecentTrend), 7)
	}

	updateResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(instance.ID), map[string]any{
		"name":             "mysql-main",
		"source_type":      "ssh",
		"source_path":      "/data/mysql",
		"exclude_patterns": []string{"node_modules/", "*.tmp"},
		"remote_config_id": remote.ID,
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/instances/{id} status = %d, want %d, body = %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}

	updated, err := db.GetInstanceByID(instance.ID)
	if err != nil {
		t.Fatalf("GetInstanceByID(updated) error = %v", err)
	}
	if updated.Name != "mysql-main" || updated.SourcePath != "/data/mysql" {
		t.Fatalf("updated instance = %+v, want renamed instance", updated)
	}
	if len(updated.ExcludePatterns) != 2 || updated.ExcludePatterns[0] != "node_modules/" || updated.ExcludePatterns[1] != "*.tmp" {
		t.Fatalf("updated.ExcludePatterns = %#v, want updated patterns", updated.ExcludePatterns)
	}

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/instances/"+itoa(instance.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/instances/{id} status = %d, want %d, body = %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	assertRowCount(t, db, `SELECT COUNT(*) FROM policies WHERE instance_id = ?`, instance.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM backups WHERE instance_id = ?`, instance.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM tasks WHERE instance_id = ?`, instance.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM instance_permissions WHERE instance_id = ?`, instance.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM notification_subscriptions WHERE instance_id = ?`, instance.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE instance_id = ?`, instance.ID, 0)
}

func TestInstanceValidationAndIdleRestrictions(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"))

	missingRemote := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances", map[string]any{
		"name":        "ssh-no-remote",
		"source_type": "ssh",
		"source_path": "/srv/mysql",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, missingRemote, http.StatusBadRequest, authErrorInvalidRequest, "remote_config_id is required for ssh source")

	invalidLocalRemote := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances", map[string]any{
		"name":             "local-with-remote",
		"source_type":      "local",
		"source_path":      "/srv/mysql",
		"remote_config_id": 1,
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, invalidLocalRemote, http.StatusBadRequest, authErrorInvalidRequest, "remote_config_id is only supported for ssh source")

	running := &model.Instance{
		Name:       "running-instance",
		SourceType: "local",
		SourcePath: "/srv/running",
		Status:     "running",
	}
	if err := db.CreateInstance(running); err != nil {
		t.Fatalf("CreateInstance(running) error = %v", err)
	}

	updateRunning := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(running.ID), map[string]any{
		"name":        "running-instance-renamed",
		"source_type": "local",
		"source_path": "/srv/running",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, updateRunning, http.StatusBadRequest, authErrorInvalidRequest, "only idle instances can be edited")

	deleteRunning := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/instances/"+itoa(running.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, deleteRunning, http.StatusBadRequest, authErrorInvalidRequest, "only idle instances can be deleted")

	permissionsResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/instances/"+itoa(running.ID)+"/permissions", map[string]any{
		"permissions": []map[string]any{{
			"user_id":    admin.ID,
			"permission": "readonly",
		}},
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, permissionsResponse, http.StatusBadRequest, authErrorInvalidRequest, "user_id "+itoa(admin.ID)+" must belong to a viewer")

	viewerWithoutPermission := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if viewerWithoutPermission.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/instances viewer status = %d, want %d", viewerWithoutPermission.Code, http.StatusOK)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(viewerWithoutPermission.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode empty viewer list response: %v", err)
	}
	var page struct {
		Items []instanceListItem `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(envelope.Data, &page); err != nil {
		t.Fatalf("decode empty viewer list payload: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("viewer page = %+v, want empty list", page)
	}
}

func createHandlerTestPolicy(t *testing.T, db *store.DB, instanceID, targetID int64, name string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO policies (instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retention_type, retention_value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		instanceID,
		name,
		"rolling",
		targetID,
		"cron",
		"0 0 * * *",
		true,
		false,
		false,
		nil,
		false,
		nil,
		"count",
		7,
	)
	if err != nil {
		t.Fatalf("insert policy error = %v", err)
	}

	policyID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(policy) error = %v", err)
	}

	return policyID
}

func insertHandlerTestBackup(t *testing.T, db *store.DB, instanceID, policyID int64, status string, backupSize, actualSize int64, completedAtExpr string) int64 {
	t.Helper()

	query := `INSERT INTO backups (instance_id, policy_id, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ` + completedAtExpr + `, ` + completedAtExpr + `, ?, ?, ?, ` + completedAtExpr + `)`
	result, err := db.Exec(
		query,
		instanceID,
		policyID,
		"rolling",
		status,
		"/snapshots/"+status,
		backupSize,
		actualSize,
		60,
		"",
		`{"files":10}`,
	)
	if err != nil {
		t.Fatalf("insert backup error = %v", err)
	}

	backupID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(backup) error = %v", err)
	}

	return backupID
}

func insertHandlerTestTask(t *testing.T, db *store.DB, instanceID, backupID int64) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO tasks (instance_id, backup_id, type, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)`,
		instanceID,
		backupID,
		"backup",
		"success",
		100,
		"done",
		"",
	); err != nil {
		t.Fatalf("insert task error = %v", err)
	}
}

func insertHandlerTestNotificationSubscription(t *testing.T, db *store.DB, userID, instanceID int64) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO notification_subscriptions (user_id, instance_id, enabled, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`, userID, instanceID, true); err != nil {
		t.Fatalf("insert notification subscription error = %v", err)
	}
}

func insertHandlerTestAuditLog(t *testing.T, db *store.DB, userID, instanceID int64) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, instanceID, userID, "instance.updated", "seed"); err != nil {
		t.Fatalf("insert audit log error = %v", err)
	}
}
