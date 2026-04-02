package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
)

func TestNotificationChannelCRUDAndSubscriptionAPI(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	adminToken := loginForAccessToken(t, router, "admin", "secret")
	viewer := createAPITestUser(t, fixture.db, "notify-viewer", "viewer-secret", false)
	viewerToken := loginForAccessToken(t, router, viewer.Username, "viewer-secret")

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      service.SourceTypeLocal,
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}
	permission := model.InstancePermission{UserID: viewer.ID, InstanceID: instance.ID, Role: service.RoleViewer}
	if err := fixture.db.Create(&permission).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/notification-channels", bytes.NewBufferString(`{"name":"smtp-main","type":"smtp","config":{"host":"smtp.example.com","port":587,"username":"mailer","password":"super-secret","from":"backup@example.com","tls":true},"enabled":true}`))
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}
	if strings.Contains(createResp.Body.String(), "super-secret") {
		t.Fatal("expected notification channel response to omit the smtp password")
	}

	var createdChannel struct {
		ID     uint                   `json:"id"`
		Name   string                 `json:"name"`
		Config map[string]any         `json:"config"`
		Enabled bool                  `json:"enabled"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdChannel); err != nil {
		t.Fatalf("decode notification channel response: %v", err)
	}
	if createdChannel.ID == 0 || createdChannel.Name != "smtp-main" || !createdChannel.Enabled {
		t.Fatalf("unexpected notification channel response: %+v", createdChannel)
	}
	if hasPassword, ok := createdChannel.Config["has_password"].(bool); !ok || !hasPassword {
		t.Fatalf("expected sanitized config to report has_password=true, got %+v", createdChannel.Config)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/notification-channels", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.Code)
	}

	viewerListReq := httptest.NewRequest(http.MethodGet, "/api/notification-channels", nil)
	viewerListReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerListResp := httptest.NewRecorder()
	router.ServeHTTP(viewerListResp, viewerListReq)
	if viewerListResp.Code != http.StatusOK {
		t.Fatalf("expected viewer channel list to return 200, got %d", viewerListResp.Code)
	}
	var viewerChannels []struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(viewerListResp.Body).Decode(&viewerChannels); err != nil {
		t.Fatalf("decode viewer channel list: %v", err)
	}
	if len(viewerChannels) != 1 || viewerChannels[0].ID != createdChannel.ID {
		t.Fatalf("expected viewers to see the enabled notification channel, got %+v", viewerChannels)
	}
	if strings.Contains(viewerListResp.Body.String(), "smtp.example.com") {
		t.Fatal("expected viewer channel list to omit smtp host details")
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/notification-channels/"+strconv.FormatUint(uint64(createdChannel.ID), 10), bytes.NewBufferString(`{"name":"smtp-updated","type":"smtp","config":{"host":"smtp.example.com","port":587,"username":"mailer","from":"backup@example.com","tls":true},"enabled":false}`))
	updateReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateResp.Code)
	}

	var storedChannel model.NotificationChannel
	if err := fixture.db.First(&storedChannel, createdChannel.ID).Error; err != nil {
		t.Fatalf("load stored notification channel: %v", err)
	}
	if !strings.Contains(storedChannel.Config, "super-secret") {
		t.Fatalf("expected smtp password to remain stored after masked update, got %q", storedChannel.Config)
	}

	createSubscriptionReq := httptest.NewRequest(http.MethodPost, "/api/instances/"+strconv.FormatUint(uint64(instance.ID), 10)+"/subscriptions", bytes.NewBufferString(`{"channel_id":`+strconv.FormatUint(uint64(createdChannel.ID), 10)+`,"events":["backup_failed"],"channel_config":{"email":"viewer@example.com"},"enabled":true}`))
	createSubscriptionReq.Header.Set("Authorization", "Bearer "+viewerToken)
	createSubscriptionReq.Header.Set("Content-Type", "application/json")
	createSubscriptionResp := httptest.NewRecorder()
	router.ServeHTTP(createSubscriptionResp, createSubscriptionReq)
	if createSubscriptionResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createSubscriptionResp.Code)
	}

	var createdSubscription struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(createSubscriptionResp.Body).Decode(&createdSubscription); err != nil {
		t.Fatalf("decode notification subscription response: %v", err)
	}
	if createdSubscription.ID == 0 {
		t.Fatal("expected created subscription id")
	}

	listSubscriptionReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(instance.ID), 10)+"/subscriptions", nil)
	listSubscriptionReq.Header.Set("Authorization", "Bearer "+viewerToken)
	listSubscriptionResp := httptest.NewRecorder()
	router.ServeHTTP(listSubscriptionResp, listSubscriptionReq)
	if listSubscriptionResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listSubscriptionResp.Code)
	}

	var subscriptions []struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(listSubscriptionResp.Body).Decode(&subscriptions); err != nil {
		t.Fatalf("decode subscriptions list response: %v", err)
	}
	if len(subscriptions) != 1 || subscriptions[0].ID != createdSubscription.ID {
		t.Fatalf("expected the created subscription to be listed, got %+v", subscriptions)
	}

	deleteSubscriptionReq := httptest.NewRequest(http.MethodDelete, "/api/subscriptions/"+strconv.FormatUint(uint64(createdSubscription.ID), 10), nil)
	deleteSubscriptionReq.Header.Set("Authorization", "Bearer "+viewerToken)
	deleteSubscriptionResp := httptest.NewRecorder()
	router.ServeHTTP(deleteSubscriptionResp, deleteSubscriptionReq)
	if deleteSubscriptionResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteSubscriptionResp.Code)
	}

	deleteChannelReq := httptest.NewRequest(http.MethodDelete, "/api/notification-channels/"+strconv.FormatUint(uint64(createdChannel.ID), 10), nil)
	deleteChannelReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteChannelResp := httptest.NewRecorder()
	router.ServeHTTP(deleteChannelResp, deleteChannelReq)
	if deleteChannelResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteChannelResp.Code)
	}
}

func TestAuditLogsAPIListsFilteredPage(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	adminToken := loginForAccessToken(t, router, "admin", "secret")
	viewer := createAPITestUser(t, fixture.db, "audit-viewer", "viewer-secret", false)
	baseTime := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)

	logs := []model.AuditLog{
		{UserID: fixture.admin.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 1, Detail: `{"seq":1}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(1 * time.Minute)},
		{UserID: fixture.admin.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 2, Detail: `{"seq":2}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(2 * time.Minute)},
		{UserID: viewer.ID, Action: "instances.create", ResourceType: "backup_instances", ResourceID: 3, Detail: `{"seq":3}`, IPAddress: "127.0.0.1", CreatedAt: baseTime.Add(3 * time.Minute)},
	}
	if err := fixture.db.Create(&logs).Error; err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit-logs?user_id="+strconv.FormatUint(uint64(fixture.admin.ID), 10)+"&action=instances.create&page=2&page_size=1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		Items []struct {
			ResourceID uint   `json:"resource_id"`
			Action     string `json:"action"`
			Username   string `json:"username"`
		} `json:"items"`
		Total int64 `json:"total"`
		Page int   `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode audit logs response: %v", err)
	}
	if body.Total != 2 || body.Page != 2 {
		t.Fatalf("expected total=2 page=2, got total=%d page=%d", body.Total, body.Page)
	}
	if len(body.Items) != 1 || body.Items[0].ResourceID != 1 || body.Items[0].Username != fixture.admin.Username {
		t.Fatalf("unexpected audit logs page: %+v", body.Items)
	}
}