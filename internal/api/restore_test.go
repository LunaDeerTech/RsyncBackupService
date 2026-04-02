package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"gorm.io/gorm"
)

func TestRestoreAPIRequiresVerifyToken(t *testing.T) {
	router, fixture := newTask07TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	requestBody := bytes.NewBufferString(`{"backup_record_id":` + strconv.FormatUint(uint64(fixture.archiveRecord.ID), 10) + `,"restore_target_path":"` + filepath.Join(t.TempDir(), "restore-target") + `","overwrite":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/instances/"+strconv.FormatUint(uint64(fixture.instance.ID), 10)+"/restore", requestBody)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestBackupHistoryAndRestoreAPIs(t *testing.T) {
	router, fixture := newTask07TestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	backupsReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(fixture.instance.ID), 10)+"/backups", nil)
	backupsReq.Header.Set("Authorization", "Bearer "+accessToken)
	backupsResp := httptest.NewRecorder()
	router.ServeHTTP(backupsResp, backupsReq)
	if backupsResp.Code != http.StatusOK {
		t.Fatalf("expected backup history 200, got %d", backupsResp.Code)
	}

	var backups []struct {
		ID         uint   `json:"id"`
		BackupType string `json:"backup_type"`
	}
	if err := json.NewDecoder(backupsResp.Body).Decode(&backups); err != nil {
		t.Fatalf("decode backups response: %v", err)
	}
	if len(backups) != 4 {
		t.Fatalf("expected 4 backup history records, got %d", len(backups))
	}

	restorableReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(fixture.instance.ID), 10)+"/snapshots", nil)
	restorableReq.Header.Set("Authorization", "Bearer "+accessToken)
	restorableResp := httptest.NewRecorder()
	router.ServeHTTP(restorableResp, restorableReq)
	if restorableResp.Code != http.StatusOK {
		t.Fatalf("expected restorable list 200, got %d", restorableResp.Code)
	}

	var restorable []struct {
		ID         uint   `json:"id"`
		BackupType string `json:"backup_type"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(restorableResp.Body).Decode(&restorable); err != nil {
		t.Fatalf("decode restorable response: %v", err)
	}
	if len(restorable) != 2 {
		t.Fatalf("expected 2 restorable records, got %d", len(restorable))
	}

	requestBody := bytes.NewBufferString(`{"backup_record_id":` + strconv.FormatUint(uint64(fixture.archiveRecord.ID), 10) + `,"restore_target_path":"` + filepath.Join(t.TempDir(), "restore-target") + `","overwrite":false}`)
	restoreReq := httptest.NewRequest(http.MethodPost, "/api/instances/"+strconv.FormatUint(uint64(fixture.instance.ID), 10)+"/restore", requestBody)
	restoreReq.Header.Set("Authorization", "Bearer "+accessToken)
	restoreReq.Header.Set("Content-Type", "application/json")
	restoreReq.Header.Set("X-Verify-Token", issueAPIVerifyToken(t, router, accessToken, "secret"))
	restoreResp := httptest.NewRecorder()
	router.ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusCreated {
		t.Fatalf("expected restore create 201, got %d", restoreResp.Code)
	}

	var restoreBody struct {
		ID             uint `json:"id"`
		BackupRecordID uint `json:"backup_record_id"`
	}
	if err := json.NewDecoder(restoreResp.Body).Decode(&restoreBody); err != nil {
		t.Fatalf("decode restore response: %v", err)
	}
	if restoreBody.ID == 0 || restoreBody.BackupRecordID != fixture.archiveRecord.ID {
		t.Fatalf("unexpected restore response: %+v", restoreBody)
	}

	restoreRecordsReq := httptest.NewRequest(http.MethodGet, "/api/restore-records", nil)
	restoreRecordsReq.Header.Set("Authorization", "Bearer "+accessToken)
	restoreRecordsResp := httptest.NewRecorder()
	router.ServeHTTP(restoreRecordsResp, restoreRecordsReq)
	if restoreRecordsResp.Code != http.StatusOK {
		t.Fatalf("expected restore records 200, got %d", restoreRecordsResp.Code)
	}

	var restoreRecords []struct {
		ID             uint `json:"id"`
		BackupRecordID uint `json:"backup_record_id"`
	}
	if err := json.NewDecoder(restoreRecordsResp.Body).Decode(&restoreRecords); err != nil {
		t.Fatalf("decode restore records response: %v", err)
	}
	if len(restoreRecords) != 1 || restoreRecords[0].BackupRecordID != fixture.archiveRecord.ID {
		t.Fatalf("unexpected restore records: %+v", restoreRecords)
	}
}

type task07APITestFixture struct {
	db           *gorm.DB
	instance     model.BackupInstance
	archiveRecord model.BackupRecord
}

func newTask07TestRouter(t *testing.T) (http.Handler, task07APITestFixture) {
	t.Helper()

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret", JWTSecret: "task07-jwt", DataDir: t.TempDir()}
	if err := repository.MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	authService := service.NewAuthService(db, cfg.JWTSecret)
	instanceService := service.NewInstanceService(db)
	sshKeyService := service.NewSSHKeyService(db)
	storageTargetService := service.NewStorageTargetService(db)
	strategyService := service.NewStrategyService(db)
	userService := service.NewUserService(db, authService)
	permissionService := service.NewPermissionService(db)
	auditRepo := repository.NewAuditLogRepository(db)
	runner := &task07APIRunnerSpy{}
	executorService := service.NewExecutorService(db, cfg, runner, executorpkg.NewTaskManager())
	restoreService := service.NewRestoreService(db, cfg, runner, authService)

	var admin model.User
	if err := db.Where("username = ?", cfg.AdminUser).First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}

	instance := model.BackupInstance{
		Name:            "db-prod",
		SourceType:      service.SourceTypeLocal,
		SourcePath:      filepath.Join(t.TempDir(), "source"),
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	target := model.StorageTarget{
		Name:     "cold-target",
		Type:     service.StorageTargetTypeColdLocal,
		BasePath: filepath.Join(t.TempDir(), "cold-target"),
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}

	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	rollingRecord := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        service.BackupTypeRolling,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: "cold_local|ignored",
		SnapshotPath:      filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance), "20260402T120000Z"),
		VolumeCount:       1,
		StartedAt:         now.Add(-2 * time.Minute),
		FinishedAt:        &now,
	}
	if err := db.Create(&rollingRecord).Error; err != nil {
		t.Fatalf("create rolling record: %v", err)
	}
	if err := os.MkdirAll(rollingRecord.SnapshotPath, 0o755); err != nil {
		t.Fatalf("create rolling snapshot dir: %v", err)
	}

	archivePath := filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance), "db-prod_20260402T120500Z.tar.gz")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	archiveRecord := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        service.BackupTypeCold,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: "cold_local|ignored",
		SnapshotPath:      archivePath,
		VolumeCount:       1,
		StartedAt:         now.Add(-time.Minute),
		FinishedAt:        &now,
	}
	if err := db.Create(&archiveRecord).Error; err != nil {
		t.Fatalf("create archive record: %v", err)
	}

	staleArchiveRecord := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        service.BackupTypeCold,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: "cold_local|ignored",
		SnapshotPath:      filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance), "db-prod_20260402T115500Z.tar.gz"),
		VolumeCount:       1,
		StartedAt:         now.Add(-90 * time.Second),
		FinishedAt:        &now,
	}
	if err := db.Create(&staleArchiveRecord).Error; err != nil {
		t.Fatalf("create stale archive record: %v", err)
	}

	failedRecord := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        service.BackupTypeCold,
		Status:            model.BackupStatusFailed,
		TargetLocationKey: "cold_local|ignored",
		SnapshotPath:      filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance), "db-prod_failed.tar.gz"),
		VolumeCount:       1,
		StartedAt:         now.Add(-30 * time.Second),
		FinishedAt:        &now,
	}
	if err := db.Create(&failedRecord).Error; err != nil {
		t.Fatalf("create failed record: %v", err)
	}

	router := NewRouter(Dependencies{
		AuthService:          authService,
		InstanceService:      instanceService,
		SSHKeyService:        sshKeyService,
		StorageTargetService: storageTargetService,
		StrategyService:      strategyService,
		UserService:          userService,
		PermissionService:    permissionService,
		AuditLogRepo:         auditRepo,
		ExecutorService:      executorService,
		RestoreService:       restoreService,
	})

	return router, task07APITestFixture{
		db:            db,
		instance:      instance,
		archiveRecord: archiveRecord,
	}
}

type task07APIRunnerSpy struct{}

func (r *task07APIRunnerSpy) Run(_ context.Context, _ executorpkg.CommandSpec, _ func(string)) error {
	return nil
}