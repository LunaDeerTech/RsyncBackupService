package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
)

func TestTaskListDetailAndCancel(t *testing.T) {
	db := newAuthTestDB(t)
	queue := engine.NewTaskQueue(8, db)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	outsider := createHandlerTestUser(t, db, "outsider@example.com", "Outsider", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(queue))

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{
		UserID:     viewer.ID,
		Permission: "readonly",
	}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	target := &model.BackupTarget{
		Name:          "rolling-target",
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "hourly-rolling",
		Type:           "rolling",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	_, task, err := db.CreatePendingPolicyRun(policy)
	if err != nil {
		t.Fatalf("CreatePendingPolicyRun() error = %v", err)
	}
	if err := queue.Enqueue(task); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	listResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/tasks", nil, mustAccessTokenForUser(t, admin, "secret"))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks status = %d, want %d, body = %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	var listEnvelope apiEnvelope
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode task list response: %v", err)
	}
	var listPayload struct {
		Items []taskResponse `json:"items"`
	}
	if err := json.Unmarshal(listEnvelope.Data, &listPayload); err != nil {
		t.Fatalf("decode task list payload: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != task.ID {
		t.Fatalf("task list payload = %+v, want queued task %d", listPayload, task.ID)
	}
	if listPayload.Items[0].InstanceName != "mysql-prod" {
		t.Fatalf("task list instance_name = %q, want mysql-prod", listPayload.Items[0].InstanceName)
	}

	detailResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/tasks/"+itoa(task.ID), nil, mustAccessTokenForUser(t, viewer, "secret"))
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks/{id} status = %d, want %d, body = %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}

	forbiddenDetail := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/tasks/"+itoa(task.ID), nil, mustAccessTokenForUser(t, outsider, "secret"))
	assertAPIError(t, forbiddenDetail, http.StatusForbidden, 40301, "forbidden")

	cancelResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/"+itoa(task.ID)+"/cancel", nil, mustAccessTokenForUser(t, admin, "secret"))
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/tasks/{id}/cancel status = %d, want %d, body = %s", cancelResponse.Code, http.StatusOK, cancelResponse.Body.String())
	}

	loadedTask, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID(cancelled) error = %v", err)
	}
	if loadedTask.Status != "cancelled" {
		t.Fatalf("cancelled task status = %q, want cancelled", loadedTask.Status)
	}

	adminOnlyList := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/tasks", nil, mustAccessTokenForUser(t, viewer, "secret"))
	assertAPIError(t, adminOnlyList, http.StatusForbidden, 40301, "forbidden")
}