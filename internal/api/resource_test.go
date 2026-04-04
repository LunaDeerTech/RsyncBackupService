package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
)

func TestResourceAPIRejectsMissingJWT(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestStorageTargetAPIRejectsNonAdmin(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	regularUser := createAPITestUser(t, fixture.db, "operator", "password", false)
	accessToken := loginForAccessToken(t, router, regularUser.Username, "password")

	req := httptest.NewRequest(http.MethodGet, "/api/storage-targets", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
}

func TestInstanceAPIRejectsUserWithoutInstancePermission(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	regularUser := createAPITestUser(t, fixture.db, "viewer", "password", false)
	accessToken := loginForAccessToken(t, router, regularUser.Username, "password")

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create backup instance: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/instances/"+strings.TrimSpace(strconv.FormatUint(uint64(instance.ID), 10)), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
}

func TestInstanceRoutesEnforcePermissionsAndDeleteVerifyToken(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	creator := createAPITestUser(t, fixture.db, "creator", "creator-secret", false)
	viewer := createAPITestUser(t, fixture.db, "viewer-admin-check", "viewer-secret", false)
	creatorAccessToken := loginForAccessToken(t, router, creator.Username, "creator-secret")
	viewerAccessToken := loginForAccessToken(t, router, viewer.Username, "viewer-secret")

	createReq := httptest.NewRequest(http.MethodPost, "/api/instances", bytes.NewBufferString(`{"name":"instance-a","source_type":"local","source_path":"`+filepath.Join(t.TempDir(), "source")+`","enabled":true}`))
	createReq.Header.Set("Authorization", "Bearer "+creatorAccessToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}

	var createdBody struct {
		ID        uint `json:"id"`
		CreatedBy uint `json:"created_by"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createdBody.ID == 0 {
		t.Fatal("expected created instance id")
	}
	if createdBody.CreatedBy != creator.ID {
		t.Fatalf("expected created_by %d, got %d", creator.ID, createdBody.CreatedBy)
	}

	var creatorPermission model.InstancePermission
	if err := fixture.db.Where("user_id = ? AND instance_id = ?", creator.ID, createdBody.ID).First(&creatorPermission).Error; err != nil {
		t.Fatalf("load creator permission: %v", err)
	}
	if creatorPermission.Role != service.RoleAdmin {
		t.Fatalf("expected creator role %q, got %q", service.RoleAdmin, creatorPermission.Role)
	}

	viewerPermission := model.InstancePermission{UserID: viewer.ID, InstanceID: createdBody.ID, Role: service.RoleViewer}
	if err := fixture.db.Create(&viewerPermission).Error; err != nil {
		t.Fatalf("create viewer permission: %v", err)
	}

	creatorGetReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	creatorGetReq.Header.Set("Authorization", "Bearer "+creatorAccessToken)
	creatorGetResp := httptest.NewRecorder()
	router.ServeHTTP(creatorGetResp, creatorGetReq)
	if creatorGetResp.Code != http.StatusOK {
		t.Fatalf("expected creator get to return 200, got %d", creatorGetResp.Code)
	}

	viewerGetReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	viewerGetReq.Header.Set("Authorization", "Bearer "+viewerAccessToken)
	viewerGetResp := httptest.NewRecorder()
	router.ServeHTTP(viewerGetResp, viewerGetReq)
	if viewerGetResp.Code != http.StatusOK {
		t.Fatalf("expected viewer get to return 200, got %d", viewerGetResp.Code)
	}

	viewerUpdateReq := httptest.NewRequest(http.MethodPut, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), bytes.NewBufferString(`{"name":"viewer-update","source_type":"local","source_path":"/srv/viewer-update","enabled":false}`))
	viewerUpdateReq.Header.Set("Authorization", "Bearer "+viewerAccessToken)
	viewerUpdateReq.Header.Set("Content-Type", "application/json")
	viewerUpdateResp := httptest.NewRecorder()
	router.ServeHTTP(viewerUpdateResp, viewerUpdateReq)
	if viewerUpdateResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer update to return 403, got %d", viewerUpdateResp.Code)
	}

	creatorUpdateReq := httptest.NewRequest(http.MethodPut, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), bytes.NewBufferString(`{"name":"creator-update","source_type":"local","source_path":"/srv/creator-update","enabled":false}`))
	creatorUpdateReq.Header.Set("Authorization", "Bearer "+creatorAccessToken)
	creatorUpdateReq.Header.Set("Content-Type", "application/json")
	creatorUpdateResp := httptest.NewRecorder()
	router.ServeHTTP(creatorUpdateResp, creatorUpdateReq)
	if creatorUpdateResp.Code != http.StatusOK {
		t.Fatalf("expected creator update to return 200, got %d", creatorUpdateResp.Code)
	}

	var storedInstance model.BackupInstance
	if err := fixture.db.First(&storedInstance, createdBody.ID).Error; err != nil {
		t.Fatalf("load updated instance: %v", err)
	}
	if storedInstance.Name != "creator-update" || storedInstance.SourcePath != "/srv/creator-update" || storedInstance.Enabled {
		t.Fatalf("expected persisted instance update, got %+v", storedInstance)
	}

	viewerDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	viewerDeleteReq.Header.Set("Authorization", "Bearer "+viewerAccessToken)
	viewerDeleteReq.Header.Set("X-Verify-Token", issueAPIVerifyToken(t, router, viewerAccessToken, "viewer-secret"))
	viewerDeleteResp := httptest.NewRecorder()
	router.ServeHTTP(viewerDeleteResp, viewerDeleteReq)
	if viewerDeleteResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer delete to return 403, got %d", viewerDeleteResp.Code)
	}

	creatorDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	creatorDeleteReq.Header.Set("Authorization", "Bearer "+creatorAccessToken)
	creatorDeleteResp := httptest.NewRecorder()
	router.ServeHTTP(creatorDeleteResp, creatorDeleteReq)
	if creatorDeleteResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected delete without verify token to return 401, got %d", creatorDeleteResp.Code)
	}

	verifiedDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/instances/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	verifiedDeleteReq.Header.Set("Authorization", "Bearer "+creatorAccessToken)
	verifiedDeleteReq.Header.Set("X-Verify-Token", issueAPIVerifyToken(t, router, creatorAccessToken, "creator-secret"))
	verifiedDeleteResp := httptest.NewRecorder()
	router.ServeHTTP(verifiedDeleteResp, verifiedDeleteReq)
	if verifiedDeleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected creator delete with verify token to return 204, got %d", verifiedDeleteResp.Code)
	}
}

func TestStrategyListIncludesUpcomingRunsPreview(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")

	instance := model.BackupInstance{
		Name:            "preview-instance",
		SourceType:      "local",
		SourcePath:      "/srv/preview-source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	target := model.StorageTarget{
		Name:     "preview-target",
		Type:     service.StorageTargetTypeRollingLocal,
		BasePath: filepath.Join(t.TempDir(), "preview-target"),
	}
	if err := fixture.db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/instances/"+strconv.FormatUint(uint64(instance.ID), 10)+"/strategies", bytes.NewBufferString(`{"name":"quarter-hourly","backup_type":"rolling","interval_seconds":900,"retention_days":7,"retention_count":3,"max_execution_seconds":3600,"storage_target_ids":[`+strconv.FormatUint(uint64(target.ID), 10)+`],"enabled":true}`))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")

	beforeCreate := time.Now().UTC()
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}
	afterCreate := time.Now().UTC()

	listReq := httptest.NewRequest(http.MethodGet, "/api/instances/"+strconv.FormatUint(uint64(instance.ID), 10)+"/strategies", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.Code)
	}

	var strategies []struct {
		ID           uint     `json:"id"`
		UpcomingRuns []string `json:"upcoming_runs"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&strategies); err != nil {
		t.Fatalf("decode strategies response: %v", err)
	}
	if len(strategies) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(strategies))
	}
	if len(strategies[0].UpcomingRuns) != 10 {
		t.Fatalf("expected 10 upcoming runs, got %d", len(strategies[0].UpcomingRuns))
	}

	firstRun, err := time.Parse(http.TimeFormat, strategies[0].UpcomingRuns[0])
	if err != nil {
		t.Fatalf("parse first upcoming run: %v", err)
	}
	if firstRun.Before(beforeCreate.Add(14*time.Minute)) || firstRun.After(afterCreate.Add(16*time.Minute)) {
		t.Fatalf("expected first upcoming run to be about 15 minutes after registration, got %s", firstRun.Format(time.RFC3339))
	}
}

func TestStorageTargetCRUDAndConnectionTest(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	storagePath := filepath.Join(t.TempDir(), "rolling-target")

	createReq := httptest.NewRequest(http.MethodPost, "/api/storage-targets", bytes.NewBufferString(`{"name":"primary","type":"rolling_local","base_path":"`+storagePath+`"}`))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}

	var createdBody struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		BasePath string `json:"base_path"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createdBody.ID == 0 || createdBody.Name != "primary" || createdBody.BasePath != storagePath {
		t.Fatalf("unexpected create response: %+v", createdBody)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/storage-targets", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.Code)
	}

	var listedTargets []struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listedTargets); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listedTargets) != 1 || listedTargets[0].ID != createdBody.ID {
		t.Fatalf("expected listed target %d, got %+v", createdBody.ID, listedTargets)
	}

	updatedPath := filepath.Join(t.TempDir(), "rolling-target-updated")
	updateReq := httptest.NewRequest(http.MethodPut, "/api/storage-targets/"+strconv.FormatUint(uint64(createdBody.ID), 10), bytes.NewBufferString(`{"name":"archive","type":"rolling_local","base_path":"`+updatedPath+`"}`))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateResp.Code)
	}

	var updatedBody struct {
		Name     string `json:"name"`
		BasePath string `json:"base_path"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updatedBody); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updatedBody.Name != "archive" || updatedBody.BasePath != updatedPath {
		t.Fatalf("unexpected update response: %+v", updatedBody)
	}

	var storedTarget model.StorageTarget
	if err := fixture.db.First(&storedTarget, createdBody.ID).Error; err != nil {
		t.Fatalf("load updated storage target: %v", err)
	}
	if storedTarget.Name != "archive" || storedTarget.BasePath != updatedPath {
		t.Fatalf("expected persisted storage target update, got %+v", storedTarget)
	}

	testReq := httptest.NewRequest(http.MethodPost, "/api/storage-targets/"+strconv.FormatUint(uint64(createdBody.ID), 10)+"/test", nil)
	testReq.Header.Set("Authorization", "Bearer "+accessToken)
	testResp := httptest.NewRecorder()
	router.ServeHTTP(testResp, testReq)
	if testResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", testResp.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/storage-targets/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteResp.Code)
	}

	var remainingTargets int64
	if err := fixture.db.Model(&model.StorageTarget{}).Where("id = ?", createdBody.ID).Count(&remainingTargets).Error; err != nil {
		t.Fatalf("count deleted storage target: %v", err)
	}
	if remainingTargets != 0 {
		t.Fatalf("expected storage target %d to be deleted, found %d rows", createdBody.ID, remainingTargets)
	}
}

func TestStorageTargetConnectionRejectsInvalidPermissionsWithoutLeakingPrivateKeyPath(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	privateKeyContent := string(generateAPIResourceTestPrivateKeyPEM(t))

	createSSHKeyReq := httptest.NewRequest(http.MethodPost, "/api/ssh-keys", marshalSSHKeyCreateRequest(t, "prod", privateKeyContent))
	createSSHKeyReq.Header.Set("Authorization", "Bearer "+accessToken)
	createSSHKeyReq.Header.Set("Content-Type", "application/json")
	createSSHKeyResp := httptest.NewRecorder()
	router.ServeHTTP(createSSHKeyResp, createSSHKeyReq)
	if createSSHKeyResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createSSHKeyResp.Code)
	}

	var sshKeyBody struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(createSSHKeyResp.Body).Decode(&sshKeyBody); err != nil {
		t.Fatalf("decode ssh key response: %v", err)
	}

	var storedSSHKey model.SSHKey
	if err := fixture.db.First(&storedSSHKey, sshKeyBody.ID).Error; err != nil {
		t.Fatalf("load stored ssh key: %v", err)
	}

	createTargetReq := httptest.NewRequest(http.MethodPost, "/api/storage-targets", bytes.NewBufferString(`{"name":"remote","type":"rolling_ssh","host":"127.0.0.1","user":"root","ssh_key_id":`+strconv.FormatUint(uint64(sshKeyBody.ID), 10)+`,"base_path":"/srv/backup"}`))
	createTargetReq.Header.Set("Authorization", "Bearer "+accessToken)
	createTargetReq.Header.Set("Content-Type", "application/json")
	createTargetResp := httptest.NewRecorder()
	router.ServeHTTP(createTargetResp, createTargetReq)
	if createTargetResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createTargetResp.Code)
	}

	var targetBody struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(createTargetResp.Body).Decode(&targetBody); err != nil {
		t.Fatalf("decode storage target response: %v", err)
	}

	if err := os.Chmod(storedSSHKey.PrivateKeyPath, 0o644); err != nil {
		t.Fatal("chmod managed private key")
	}

	testReq := httptest.NewRequest(http.MethodPost, "/api/storage-targets/"+strconv.FormatUint(uint64(targetBody.ID), 10)+"/test", nil)
	testReq.Header.Set("Authorization", "Bearer "+accessToken)
	testResp := httptest.NewRecorder()
	router.ServeHTTP(testResp, testReq)
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", testResp.Code)
	}
	if strings.Contains(testResp.Body.String(), storedSSHKey.PrivateKeyPath) {
		t.Fatal("expected storage target test response to omit private key path")
	}
}

func TestSSHKeyRegisterDoesNotExposePrivateKeyPath(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	privateKeyContent := string(generateAPIResourceTestPrivateKeyPEM(t))

	req := httptest.NewRequest(http.MethodPost, "/api/ssh-keys", marshalSSHKeyCreateRequest(t, "prod", privateKeyContent))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
	responseBody := resp.Body.String()
	if strings.Contains(responseBody, "BEGIN RSA PRIVATE KEY") {
		t.Fatal("expected ssh key response to omit uploaded private key material")
	}

	var body struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID == 0 || body.Name != "prod" || body.Fingerprint == "" {
		t.Fatalf("expected ssh key metadata response, got %+v", body)
	}

	var storedSSHKey model.SSHKey
	if err := fixture.db.First(&storedSSHKey, body.ID).Error; err != nil {
		t.Fatalf("load stored ssh key: %v", err)
	}
	if strings.Contains(responseBody, storedSSHKey.PrivateKeyPath) {
		t.Fatal("expected ssh key response to omit managed private key path")
	}
	privateKeyInfo, err := os.Stat(storedSSHKey.PrivateKeyPath)
	if err != nil {
		t.Fatal("stat managed private key")
	}
	if privateKeyInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected managed private key to be 0600, got %04o", privateKeyInfo.Mode().Perm())
	}
}

func TestSSHKeyCreateRejectsLegacyPrivateKeyPathPayload(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	privateKeyPath := writeAPIResourceTestPrivateKey(t, 0o600)

	req := httptest.NewRequest(http.MethodPost, "/api/ssh-keys", bytes.NewBufferString(`{"name":"prod","private_key_path":"`+privateKeyPath+`"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if strings.Contains(resp.Body.String(), privateKeyPath) {
		t.Fatal("expected legacy private key path payload to be rejected without leaking the path")
	}
	if !strings.Contains(resp.Body.String(), service.ErrPrivateKeyRequired.Error()) {
		t.Fatalf("expected private key required error, got %s", resp.Body.String())
	}
}

func TestSSHKeyConnectionRejectsInvalidPermissionsWithoutLeakingPrivateKeyPath(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	privateKeyContent := string(generateAPIResourceTestPrivateKeyPEM(t))

	createReq := httptest.NewRequest(http.MethodPost, "/api/ssh-keys", marshalSSHKeyCreateRequest(t, "prod", privateKeyContent))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}

	var createdBody struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	var storedSSHKey model.SSHKey
	if err := fixture.db.First(&storedSSHKey, createdBody.ID).Error; err != nil {
		t.Fatalf("load stored ssh key: %v", err)
	}

	if err := os.Chmod(storedSSHKey.PrivateKeyPath, 0o644); err != nil {
		t.Fatal("chmod managed private key")
	}

	testReq := httptest.NewRequest(http.MethodPost, "/api/ssh-keys/"+strconv.FormatUint(uint64(createdBody.ID), 10)+"/test", bytes.NewBufferString(`{"host":"127.0.0.1","user":"root"}`))
	testReq.Header.Set("Authorization", "Bearer "+accessToken)
	testReq.Header.Set("Content-Type", "application/json")

	testResp := httptest.NewRecorder()
	router.ServeHTTP(testResp, testReq)

	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", testResp.Code)
	}
	if strings.Contains(testResp.Body.String(), storedSSHKey.PrivateKeyPath) {
		t.Fatal("expected ssh key test response to omit private key path")
	}
}

func TestSSHKeyDeleteRemovesRegisteredKey(t *testing.T) {
	router, fixture := newAuthTestRouter(t)
	accessToken := loginForAccessToken(t, router, "admin", "secret")
	privateKeyContent := string(generateAPIResourceTestPrivateKeyPEM(t))

	createReq := httptest.NewRequest(http.MethodPost, "/api/ssh-keys", marshalSSHKeyCreateRequest(t, "prod", privateKeyContent))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.Code)
	}

	var createdBody struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	var storedSSHKey model.SSHKey
	if err := fixture.db.First(&storedSSHKey, createdBody.ID).Error; err != nil {
		t.Fatalf("load stored ssh key: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/ssh-keys", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.Code)
	}
	if strings.Contains(listResp.Body.String(), storedSSHKey.PrivateKeyPath) {
		t.Fatal("expected ssh key list response to omit private key path")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/ssh-keys/"+strconv.FormatUint(uint64(createdBody.ID), 10), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteResp.Code)
	}

	postDeleteListReq := httptest.NewRequest(http.MethodGet, "/api/ssh-keys", nil)
	postDeleteListReq.Header.Set("Authorization", "Bearer "+accessToken)
	postDeleteListResp := httptest.NewRecorder()
	router.ServeHTTP(postDeleteListResp, postDeleteListReq)
	if postDeleteListResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", postDeleteListResp.Code)
	}

	var listedSSHKeys []struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(postDeleteListResp.Body).Decode(&listedSSHKeys); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, sshKey := range listedSSHKeys {
		if sshKey.ID == createdBody.ID {
			t.Fatalf("expected ssh key %d to be deleted, got %+v", createdBody.ID, listedSSHKeys)
		}
	}
	if _, err := os.Stat(storedSSHKey.PrivateKeyPath); !os.IsNotExist(err) {
		t.Fatal("expected managed private key file to be removed")
	}
}

func issueAPIVerifyToken(t *testing.T, router http.Handler, accessToken, password string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(`{"password":"`+password+`"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body struct {
		VerifyToken string `json:"verify_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if body.VerifyToken == "" {
		t.Fatal("expected verify token")
	}

	return body.VerifyToken
}

func generateAPIResourceTestPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa private key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
}

func writeAPIResourceTestPrivateKey(t *testing.T, mode os.FileMode) string {
	t.Helper()

	encodedKey := generateAPIResourceTestPrivateKeyPEM(t)
	privateKeyPath := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(privateKeyPath, encodedKey, mode); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	return privateKeyPath
}

func marshalSSHKeyCreateRequest(t *testing.T, name, privateKey string) *bytes.Buffer {
	t.Helper()

	payload, err := json.Marshal(map[string]string{
		"name":        name,
		"private_key": privateKey,
	})
	if err != nil {
		t.Fatalf("marshal ssh key create request: %v", err)
	}

	return bytes.NewBuffer(payload)
}
