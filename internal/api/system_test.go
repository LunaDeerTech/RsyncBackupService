package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wspkg "github.com/LunaDeerTech/RsyncBackupService/internal/api/ws"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"golang.org/x/net/websocket"
	"gorm.io/gorm"
)

func TestRunningTasksEndpointReturnsInMemoryTasks(t *testing.T) {
	router, fixture := newTask09TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	task, ok := fixture.taskManager.TryStart(executorpkg.BuildTaskLockKey(12, 34), cancel)
	if !ok {
		t.Fatal("expected task manager to accept first running task")
	}
	fixture.executorService.PublishProgress(service.ProgressEvent{
		TaskID:        task.ID,
		InstanceID:    12,
		Percentage:    48.5,
		SpeedText:     "18 MB/s",
		RemainingText: "90s",
		Status:        model.BackupStatusRunning,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/running", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body []struct {
		TaskID          string  `json:"task_id"`
		InstanceID      uint    `json:"instance_id"`
		StorageTargetID uint    `json:"storage_target_id"`
		Percentage      float64 `json:"percentage"`
		SpeedText       string  `json:"speed_text"`
		RemainingText   string  `json:"remaining_text"`
		Status          string  `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode running tasks response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 running task, got %d", len(body))
	}
	if body[0].TaskID != task.ID {
		t.Fatalf("expected task id %q, got %q", task.ID, body[0].TaskID)
	}
	if body[0].InstanceID != 12 || body[0].StorageTargetID != 34 {
		t.Fatalf("expected task location 12/34, got %d/%d", body[0].InstanceID, body[0].StorageTargetID)
	}
	if body[0].Percentage != 48.5 || body[0].SpeedText != "18 MB/s" || body[0].RemainingText != "90s" {
		t.Fatalf("unexpected progress payload: %+v", body[0])
	}
	if body[0].Status != model.BackupStatusRunning {
		t.Fatalf("expected running status, got %q", body[0].Status)
	}
}

func TestCancelRunningTaskEndpointInvokesTaskManager(t *testing.T) {
	router, fixture := newTask09TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	ctx, cancel := context.WithCancel(context.Background())
	task, ok := fixture.taskManager.TryStart(executorpkg.BuildTaskLockKey(3, 4), cancel)
	if !ok {
		t.Fatal("expected task manager to accept first running task")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.Code)
	}

	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected running task context to be cancelled")
	}
}

func TestDashboardEndpointReturnsSummaryCounts(t *testing.T) {
	router, fixture := newTask09TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	now := time.Now().UTC()
	instanceA := model.BackupInstance{
		Name:            "alpha",
		SourceType:      service.SourceTypeLocal,
		SourcePath:      filepath.Join(t.TempDir(), "alpha"),
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	instanceB := model.BackupInstance{
		Name:            "beta",
		SourceType:      service.SourceTypeLocal,
		SourcePath:      filepath.Join(t.TempDir(), "beta"),
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instanceA).Error; err != nil {
		t.Fatalf("create instance alpha: %v", err)
	}
	if err := fixture.db.Create(&instanceB).Error; err != nil {
		t.Fatalf("create instance beta: %v", err)
	}

	target := model.StorageTarget{Name: "local-target", Type: service.StorageTargetTypeColdLocal, BasePath: t.TempDir()}
	if err := fixture.db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	finishedSuccess := now.Add(-10 * time.Minute)
	finishedFailure := now.Add(-5 * time.Minute)
	records := []model.BackupRecord{
		{
			InstanceID:        instanceA.ID,
			StorageTargetID:   target.ID,
			BackupType:        service.BackupTypeRolling,
			Status:            model.BackupStatusSuccess,
			TargetLocationKey: "local|alpha",
			SnapshotPath:      filepath.Join(target.BasePath, "alpha-snapshot"),
			StartedAt:         now.Add(-15 * time.Minute),
			FinishedAt:        &finishedSuccess,
		},
		{
			InstanceID:        instanceB.ID,
			StorageTargetID:   target.ID,
			BackupType:        service.BackupTypeCold,
			Status:            model.BackupStatusFailed,
			TargetLocationKey: "local|beta",
			SnapshotPath:      filepath.Join(target.BasePath, "beta-snapshot"),
			StartedAt:         now.Add(-8 * time.Minute),
			FinishedAt:        &finishedFailure,
			ErrorMessage:      "network timeout",
		},
	}
	if err := fixture.db.Create(&records).Error; err != nil {
		t.Fatalf("create backup records: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		InstanceCount    int64 `json:"instance_count"`
		TodayBackupCount int64 `json:"today_backup_count"`
		SuccessCount     int64 `json:"success_count"`
		FailedCount      int64 `json:"failed_count"`
		RecentBackups    []struct {
			ID         uint   `json:"id"`
			InstanceID uint   `json:"instance_id"`
			Status     string `json:"status"`
		} `json:"recent_backups"`
		StorageOverview []struct {
			StorageTargetID uint   `json:"storage_target_id"`
			BackupCount     int64  `json:"backup_count"`
			AvailableBytes  uint64 `json:"available_bytes"`
		} `json:"storage_overview"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode dashboard response: %v", err)
	}
	if body.InstanceCount != 2 {
		t.Fatalf("expected 2 instances, got %d", body.InstanceCount)
	}
	if body.TodayBackupCount != 2 || body.SuccessCount != 1 || body.FailedCount != 1 {
		t.Fatalf("unexpected dashboard counts: %+v", body)
	}
	if len(body.RecentBackups) != 2 {
		t.Fatalf("expected 2 recent backups, got %d", len(body.RecentBackups))
	}
	if len(body.StorageOverview) != 1 || body.StorageOverview[0].StorageTargetID != target.ID || body.StorageOverview[0].BackupCount != 2 {
		t.Fatalf("unexpected storage overview: %+v", body.StorageOverview)
	}
	if body.StorageOverview[0].AvailableBytes == 0 {
		t.Fatal("expected storage overview to include available space")
	}
}

func TestSystemStatusEndpointReturnsRuntimeAndDiskFields(t *testing.T) {
	router, fixture := newTask09TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		Version        string `json:"version"`
		DataDir        string `json:"data_dir"`
		UptimeSeconds  int64  `json:"uptime_seconds"`
		DiskTotalBytes uint64 `json:"disk_total_bytes"`
		DiskFreeBytes  uint64 `json:"disk_free_bytes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode system status response: %v", err)
	}
	if body.Version == "" {
		t.Fatal("expected version to be populated")
	}
	if body.DataDir != fixture.cfg.DataDir {
		t.Fatalf("expected data dir %q, got %q", fixture.cfg.DataDir, body.DataDir)
	}
	if body.UptimeSeconds < 0 {
		t.Fatalf("expected non-negative uptime, got %d", body.UptimeSeconds)
	}
	if body.DiskTotalBytes == 0 {
		t.Fatal("expected non-zero disk total bytes")
	}
	if body.DiskFreeBytes == 0 {
		t.Fatal("expected non-zero disk free bytes")
	}
}

func TestProgressWebSocketEndpointBroadcastsEvents(t *testing.T) {
	router, fixture := newTask09TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws/progress?access_token=" + url.QueryEscape(accessToken)
	config, err := websocket.NewConfig(wsURL, server.URL)
	if err != nil {
		t.Fatalf("build websocket config: %v", err)
	}

	conn, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatalf("dial websocket endpoint: %v", err)
	}
	defer conn.Close()

	event := service.ProgressEvent{
		TaskID:        "task-ws-1",
		InstanceID:    99,
		Percentage:    66.6,
		SpeedText:     "21 MB/s",
		RemainingText: "45s",
		Status:        model.BackupStatusRunning,
	}
	fixture.executorService.PublishProgress(event)

	var received service.ProgressEvent
	if err := websocket.JSON.Receive(conn, &received); err != nil {
		t.Fatalf("receive websocket progress event: %v", err)
	}
	if received != event {
		t.Fatalf("expected websocket event %+v, got %+v", event, received)
	}
}

type task09APITestFixture struct {
	db              *gorm.DB
	cfg             config.Config
	admin           model.User
	taskManager     *executorpkg.TaskManager
	executorService *service.ExecutorService
}

func newTask09TestRouter(t *testing.T) (http.Handler, task09APITestFixture) {
	t.Helper()

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret", JWTSecret: "task09-jwt", DataDir: t.TempDir()}
	if err := repository.MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	authService := service.NewAuthService(db, cfg.JWTSecret)
	instanceService := service.NewInstanceService(db)
	notificationService := service.NewNotificationService(db)
	sshKeyService := service.NewSSHKeyService(db, cfg.DataDir)
	storageTargetService := service.NewStorageTargetService(db)
	strategyService := service.NewStrategyService(db)
	userService := service.NewUserService(db, authService)
	permissionService := service.NewPermissionService(db)
	auditService := service.NewAuditService(db)
	auditRepo := repository.NewAuditLogRepository(db)
	taskManager := executorpkg.NewTaskManager()
	executorService := service.NewExecutorService(db, cfg, nil, taskManager, notificationService)
	dashboardService := service.NewDashboardService(db, cfg, executorService)
	progressHub := wspkg.NewHub()
	stopProgressBridge := wspkg.BridgeProgress(executorService, progressHub)
	t.Cleanup(stopProgressBridge)
	restoreService := service.NewRestoreService(db, cfg, nil, authService, notificationService)

	var admin model.User
	if err := db.Where("username = ?", cfg.AdminUser).First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}

	router := NewRouter(Dependencies{
		AuthService:          authService,
		AuditService:         auditService,
		ExecutorService:      executorService,
		DashboardService:     dashboardService,
		InstanceService:      instanceService,
		NotificationService:  notificationService,
		ProgressHub:          progressHub,
		RestoreService:       restoreService,
		SSHKeyService:        sshKeyService,
		StorageTargetService: storageTargetService,
		StrategyService:      strategyService,
		UserService:          userService,
		PermissionService:    permissionService,
		AuditLogRepo:         auditRepo,
	})

	return router, task09APITestFixture{
		db:              db,
		cfg:             cfg,
		admin:           admin,
		taskManager:     taskManager,
		executorService: executorService,
	}
}
