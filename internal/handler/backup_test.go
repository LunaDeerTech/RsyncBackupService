package handler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/store"
)

func TestRestoreBackupRejectsWrongPassword(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, false)
	response := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/restore", map[string]any{
		"restore_type":  "source",
		"instance_name": "mysql-prod",
		"password":      "WrongPass123",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, response, http.StatusBadRequest, authErrorInvalidRequest, "password is incorrect")
}

func TestRestoreBackupCreatesQueuedRestoreTask(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, true)
	response := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/restore", map[string]any{
		"restore_type":   "custom",
		"target_path":    "/restore/mysql-prod",
		"instance_name":  "mysql-prod",
		"password":       "AdminPass123",
		"encryption_key": "Cold#123",
	}, mustAccessTokenForUser(t, admin, "secret"))
	if response.Code != http.StatusCreated {
		t.Fatalf("POST restore status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Task model.Task `json:"task"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Task.Type != "restore" || payload.Task.RestoreType != "custom" || payload.Task.TargetPath != "/restore/mysql-prod" {
		t.Fatalf("restore task payload = %+v, want queued custom restore task", payload.Task)
	}
	loadedTask, err := db.GetTaskByID(payload.Task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if loadedTask.Status != "queued" {
		t.Fatalf("loaded task status = %q, want queued", loadedTask.Status)
	}
}

func TestBackupDownloadTokenSupportsRepeatedRangeRequests(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createColdBackupForHandlerTests(t, db, false)
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readdownload"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/download", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET download URL status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Mode string `json:"mode"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Mode != "single" {
		t.Fatalf("download mode = %q, want %q", payload.Mode, "single")
	}
	if payload.URL == "" {
		t.Fatal("download URL = empty, want temporary URL")
	}

	request := httptest.NewRequest(http.MethodGet, payload.URL, nil)
	request.Header.Set("Range", "bytes=0-3")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("GET token ranged download status = %d, want %d", recorder.Code, http.StatusPartialContent)
	}
	if recorder.Body.String() != "cold" {
		t.Fatalf("download body = %q, want %q", recorder.Body.String(), "cold")
	}
	if recorder.Header().Get("Content-Range") != "bytes 0-3/17" {
		t.Fatalf("Content-Range = %q, want %q", recorder.Header().Get("Content-Range"), "bytes 0-3/17")
	}
	if recorder.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want %q", recorder.Header().Get("Accept-Ranges"), "bytes")
	}

	secondRequest := httptest.NewRequest(http.MethodGet, payload.URL, nil)
	secondRequest.Header.Set("Range", "bytes=5-")
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusPartialContent {
		t.Fatalf("second ranged download status = %d, want %d", second.Code, http.StatusPartialContent)
	}
	if second.Body.String() != "backup-bytes" {
		t.Fatalf("second download body = %q, want %q", second.Body.String(), "backup-bytes")
	}
}

func TestDownloadSplitColdBackupReturnsPartList(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createSplitColdBackupForHandlerTests(t, db)
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readdownload"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/download", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET download URL status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Mode     string `json:"mode"`
		FileName string `json:"file_name"`
		Parts    []struct {
			Index     int    `json:"index"`
			Name      string `json:"name"`
			URL       string `json:"url"`
			SizeBytes int64  `json:"size_bytes"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Mode != "split" {
		t.Fatalf("download mode = %q, want %q", payload.Mode, "split")
	}
	if payload.FileName != "mysql-prod-20260407-210000.tar.gz" {
		t.Fatalf("file_name = %q, want %q", payload.FileName, "mysql-prod-20260407-210000.tar.gz")
	}
	if len(payload.Parts) != 2 {
		t.Fatalf("split part count = %d, want %d", len(payload.Parts), 2)
	}
	if payload.Parts[0].Name != "mysql-prod-20260407-210000.tar.gz.part001" {
		t.Fatalf("first part name = %q", payload.Parts[0].Name)
	}
	if payload.Parts[1].Name != "mysql-prod-20260407-210000.tar.gz.part002" {
		t.Fatalf("second part name = %q", payload.Parts[1].Name)
	}

	firstPart := httptest.NewRecorder()
	router.ServeHTTP(firstPart, httptest.NewRequest(http.MethodGet, payload.Parts[0].URL, nil))
	if firstPart.Code != http.StatusOK {
		t.Fatalf("GET first split download status = %d, want %d", firstPart.Code, http.StatusOK)
	}
	if firstPart.Body.String() != "cold-backup-part-1" {
		t.Fatalf("first split download body = %q", firstPart.Body.String())
	}
	if !strings.Contains(firstPart.Header().Get("Content-Disposition"), `filename="mysql-prod-20260407-210000.tar.gz.part001"`) {
		t.Fatalf("Content-Disposition = %q, want split part filename", firstPart.Header().Get("Content-Disposition"))
	}
}

func TestOpenListSplitBackupDownloadReturnsPartList(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer-openlist@example.com", "Viewer", "viewer", "ViewerPass123")
	token := "openlist-token"
	basePath := "/cold/mysql-prod-20260407-210000.tar.gz"
	partBodies := map[string]string{
		basePath + ".part001": "cold-backup-part-1",
		basePath + ".part002": "cold-backup-part-2",
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			writeOpenListTestJSON(t, w, http.StatusOK, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"token": token,
				},
			})
		case r.URL.Path == "/api/fs/get":
			assertOpenListTestAuth(t, r, token)
			var payload map[string]string
			decodeOpenListTestBody(t, r, &payload)
			content, ok := partBodies[payload["path"]]
			if !ok {
				writeOpenListTestJSON(t, w, http.StatusOK, map[string]any{"code": 500, "message": "object not found", "data": nil})
				return
			}
			writeOpenListTestJSON(t, w, http.StatusOK, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"path":   payload["path"],
					"name":   filepath.Base(payload["path"]),
					"size":   len(content),
					"is_dir": false,
				},
			})
		case strings.HasPrefix(r.URL.Path, "/@file/link/path/"):
			assertOpenListTestAuth(t, r, token)
			remotePath, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/@file/link/path"))
			if err != nil {
				t.Fatalf("PathUnescape(link path) error = %v", err)
			}
			if _, ok := partBodies[remotePath]; !ok {
				writeOpenListTestJSON(t, w, http.StatusOK, map[string]any{"code": 500, "message": "object not found", "data": nil})
				return
			}
			writeOpenListTestJSON(t, w, http.StatusOK, map[string]any{
				"data": server.URL + "/downloads" + openlist.EncodePathForRoute(remotePath),
			})
		case strings.HasPrefix(r.URL.Path, "/downloads/"):
			assertOpenListTestAuth(t, r, token)
			remotePath, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/downloads"))
			if err != nil {
				t.Fatalf("PathUnescape(download path) error = %v", err)
			}
			content, ok := partBodies[remotePath]
			if !ok {
				http.NotFound(w, r)
				return
			}
			http.ServeContent(w, r, filepath.Base(remotePath), time.Time{}, strings.NewReader(content))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	instanceID, backupID := createOpenListSplitColdBackupForHandlerTests(t, db, server.URL, basePath)
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readdownload"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))
	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/download", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET download URL status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Mode     string `json:"mode"`
		FileName string `json:"file_name"`
		Parts    []struct {
			Name      string `json:"name"`
			URL       string `json:"url"`
			SizeBytes int64  `json:"size_bytes"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Mode != "split" {
		t.Fatalf("download mode = %q, want %q", payload.Mode, "split")
	}
	if payload.FileName != "mysql-prod-20260407-210000.tar.gz" {
		t.Fatalf("file_name = %q, want %q", payload.FileName, "mysql-prod-20260407-210000.tar.gz")
	}
	if len(payload.Parts) != 2 {
		t.Fatalf("split part count = %d, want %d", len(payload.Parts), 2)
	}
	if payload.Parts[0].SizeBytes != int64(len("cold-backup-part-1")) {
		t.Fatalf("first part size = %d, want %d", payload.Parts[0].SizeBytes, len("cold-backup-part-1"))
	}

	request := httptest.NewRequest(http.MethodGet, payload.Parts[0].URL, nil)
	request.Header.Set("Range", "bytes=0-3")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("GET token ranged openlist download status = %d, want %d", recorder.Code, http.StatusPartialContent)
	}
	if recorder.Body.String() != "cold" {
		t.Fatalf("openlist download body = %q, want %q", recorder.Body.String(), "cold")
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), `filename="mysql-prod-20260407-210000.tar.gz.part001"`) {
		t.Fatalf("Content-Disposition = %q, want split part filename", recorder.Header().Get("Content-Disposition"))
	}
}

func TestDownloadDirectoryColdBackupReturnsTarGz(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	router := NewRouter(db, WithJWTSecret("secret"), WithTaskQueue(engine.NewTaskQueue(8, db)))

	instanceID, backupID := createDirectoryColdBackupForHandlerTests(t, db)
	if err := db.SetInstancePermissions(instanceID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readdownload"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	response := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/instances/"+itoa(instanceID)+"/backups/"+itoa(backupID)+"/download", nil, mustAccessTokenForUser(t, viewer, "secret"))
	if response.Code != http.StatusOK {
		t.Fatalf("GET download URL status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var payload struct {
		Mode string `json:"mode"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Mode != "single" {
		t.Fatalf("download mode = %q, want %q", payload.Mode, "single")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, payload.URL, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET directory download status = %d, want %d", recorder.Code, http.StatusOK)
	}
	reader, err := gzip.NewReader(strings.NewReader(recorder.Body.String()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatalf("tar Next() error = %v", err)
	}
	if header.Name != "db.sql" {
		t.Fatalf("first tar entry = %q, want %q", header.Name, "db.sql")
	}
	content, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != "directory-backup" {
		t.Fatalf("directory download content = %q, want %q", string(content), "directory-backup")
	}
}

func createColdBackupForHandlerTests(t *testing.T, db *store.DB, encrypted bool) (int64, int64) {
	t.Helper()

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
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

	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "nightly-cold",
		Type:           "cold",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		Compression:    true,
		Encryption:     encrypted,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if encrypted {
		hash := authcrypto.HashEncryptionKey("Cold#123")
		policy.EncryptionKeyHash = &hash
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	artifactPath := filepath.Join(t.TempDir(), "mysql-prod-20260407-210000.tar.gz")
	if encrypted {
		artifactPath += ".enc"
	}
	if err := os.WriteFile(artifactPath, []byte("cold-backup-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifact) error = %v", err)
	}

	completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policy.ID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            "cold",
		Status:          "success",
		SnapshotPath:    artifactPath,
		BackupSizeBytes: int64(len("cold-backup-bytes")),
		ActualSizeBytes: int64(len("cold-backup-bytes")),
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: 60,
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	return instanceID, backup.ID
}

func createSplitColdBackupForHandlerTests(t *testing.T, db *store.DB) (int64, int64) {
	t.Helper()

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
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

	partSize := 1
	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "nightly-cold-split",
		Type:           "cold",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		Compression:    true,
		SplitEnabled:   true,
		SplitSizeMB:    &partSize,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	basePath := filepath.Join(t.TempDir(), "mysql-prod-20260407-210000.tar.gz")
	if err := os.WriteFile(basePath+".part001", []byte("cold-backup-part-1"), 0o600); err != nil {
		t.Fatalf("WriteFile(part001) error = %v", err)
	}
	if err := os.WriteFile(basePath+".part002", []byte("cold-backup-part-2"), 0o600); err != nil {
		t.Fatalf("WriteFile(part002) error = %v", err)
	}

	completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policy.ID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            "cold",
		Status:          "success",
		SnapshotPath:    basePath + ".part001",
		BackupSizeBytes: int64(len("cold-backup-part-1cold-backup-part-2")),
		ActualSizeBytes: int64(len("cold-backup-part-1cold-backup-part-2")),
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: 60,
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	return instanceID, backup.ID
}

func createDirectoryColdBackupForHandlerTests(t *testing.T, db *store.DB) (int64, int64) {
	t.Helper()

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
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

	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "nightly-cold-dir",
		Type:           "cold",
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

	artifactPath := filepath.Join(t.TempDir(), "mysql-prod-20260407-210000")
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifactPath) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "db.sql"), []byte("directory-backup"), 0o600); err != nil {
		t.Fatalf("WriteFile(db.sql) error = %v", err)
	}

	completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policy.ID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            "cold",
		Status:          "success",
		SnapshotPath:    artifactPath,
		BackupSizeBytes: int64(len("directory-backup")),
		ActualSizeBytes: int64(len("directory-backup")),
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: 60,
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	return instanceID, backup.ID
}

func createOpenListSplitColdBackupForHandlerTests(t *testing.T, db *store.DB, baseURL, snapshotBasePath string) (int64, int64) {
	t.Helper()

	encodedConfig, err := openlist.EncodeStoredConfig("secret", "")
	if err != nil {
		t.Fatalf("EncodeStoredConfig() error = %v", err)
	}
	provider := "openlist"
	remote := &model.RemoteConfig{
		Name:          "openlist-remote",
		Type:          "openlist",
		Host:          baseURL,
		Username:      "admin",
		CloudProvider: &provider,
		CloudConfig:   encodedConfig,
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	instanceID := createHandlerTestInstance(t, db, "mysql-prod")
	target := &model.BackupTarget{
		Name:           "openlist-cold-target",
		BackupType:     "cold",
		StorageType:    "openlist",
		StoragePath:    "/cold",
		RemoteConfigID: &remote.ID,
		HealthStatus:   "healthy",
		HealthMessage:  "ok",
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}

	partSize := 1
	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           "nightly-openlist-split",
		Type:           "cold",
		TargetID:       target.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "3600",
		Enabled:        true,
		Compression:    true,
		SplitEnabled:   true,
		SplitSizeMB:    &partSize,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}

	completedAt := time.Date(2026, 4, 7, 21, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policy.ID,
		TriggerSource:   model.BackupTriggerSourceManual,
		Type:            "cold",
		Status:          "success",
		SnapshotPath:    snapshotBasePath + ".part001",
		BackupSizeBytes: int64(len("cold-backup-part-1cold-backup-part-2")),
		ActualSizeBytes: int64(len("cold-backup-part-1cold-backup-part-2")),
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: 60,
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	return instanceID, backup.ID
}

func writeOpenListTestJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

func decodeOpenListTestBody(t *testing.T, r *http.Request, out any) {
	t.Helper()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		t.Fatalf("json.NewDecoder().Decode() error = %v", err)
	}
}

func assertOpenListTestAuth(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
}
