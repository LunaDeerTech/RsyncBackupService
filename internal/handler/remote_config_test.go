package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestRemoteConfigCRUDFlow(t *testing.T) {
	dataDir := t.TempDir()
	db := newRemoteHandlerTestDB(t, dataDir)
	router := NewRouter(db, WithJWTSecret("secret"), WithDataDir(dataDir))

	firstKey := generatePrivateKeyPEM(t)
	createRequest := newMultipartRemoteRequest(t, http.MethodPost, "/api/v1/remotes", map[string]string{
		"name":     "db-ssh",
		"type":     "ssh",
		"host":     "192.168.0.10",
		"port":     "22",
		"username": "root",
	}, "private_key", "id_rsa.pem", firstKey)
	createRequest.Header.Set("Authorization", adminBearerToken(t, "secret"))
	createRecorder := httptest.NewRecorder()

	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body = %s", createRecorder.Code, http.StatusCreated, createRecorder.Body.String())
	}
	if strings.Contains(createRecorder.Body.String(), "private_key_path") {
		t.Fatalf("create response leaked private_key_path: %s", createRecorder.Body.String())
	}

	created, err := db.GetRemoteConfigByName("db-ssh")
	if err != nil {
		t.Fatalf("GetRemoteConfigByName() error = %v", err)
	}
	oldKeyPath := created.PrivateKeyPath
	if !strings.HasPrefix(oldKeyPath, filepath.Join(dataDir, "keys")) {
		t.Fatalf("private key path = %q, want under %q", oldKeyPath, filepath.Join(dataDir, "keys"))
	}
	info, err := os.Stat(oldKeyPath)
	if err != nil {
		t.Fatalf("Stat(old key) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("old key permissions = %#o, want %#o", info.Mode().Perm(), 0o600)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/remotes?page=1&page_size=10", nil)
	listRequest.Header.Set("Authorization", adminBearerToken(t, "secret"))
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}
	if strings.Contains(listRecorder.Body.String(), "private_key_path") {
		t.Fatalf("list response leaked private_key_path: %s", listRecorder.Body.String())
	}

	secondKey := generatePrivateKeyPEM(t)
	updateRequest := newMultipartRemoteRequest(t, http.MethodPut, "/api/v1/remotes/"+itoa(created.ID), map[string]string{
		"name": "db-ssh-updated",
	}, "private_key", "id_rsa_new.pem", secondKey)
	updateRequest.Header.Set("Authorization", adminBearerToken(t, "secret"))
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d, body = %s", updateRecorder.Code, http.StatusOK, updateRecorder.Body.String())
	}

	updated, err := db.GetRemoteConfigByID(created.ID)
	if err != nil {
		t.Fatalf("GetRemoteConfigByID(updated) error = %v", err)
	}
	if updated.Name != "db-ssh-updated" {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, "db-ssh-updated")
	}
	if updated.PrivateKeyPath == oldKeyPath {
		t.Fatal("updated.PrivateKeyPath did not change after uploading a new key")
	}
	if _, err := os.Stat(oldKeyPath); !os.IsNotExist(err) {
		t.Fatalf("old key file still exists or returned unexpected error: %v", err)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/remotes/"+itoa(created.ID), nil)
	deleteRequest.Header.Set("Authorization", adminBearerToken(t, "secret"))
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d, body = %s", deleteRecorder.Code, http.StatusOK, deleteRecorder.Body.String())
	}
	if _, err := os.Stat(updated.PrivateKeyPath); !os.IsNotExist(err) {
		t.Fatalf("updated key file still exists or returned unexpected error: %v", err)
	}
}

func TestRemoteConfigDeleteRejectsInUse(t *testing.T) {
	dataDir := t.TempDir()
	db := newRemoteHandlerTestDB(t, dataDir)
	remote := &model.RemoteConfig{
		Name:           "shared-remote",
		Type:           "cloud",
		PrivateKeyPath: "",
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, remote_config_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"mysql-prod",
		"cloud",
		"oss://bucket/mysql",
		remote.ID,
		"online",
	); err != nil {
		t.Fatalf("insert instance error = %v", err)
	}

	router := NewRouter(db, WithJWTSecret("secret"), WithDataDir(dataDir))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/remotes/"+itoa(remote.ID), nil)
	req.Header.Set("Authorization", adminBearerToken(t, "secret"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("delete status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "mysql-prod") {
		t.Fatalf("delete response = %s, want usage details", recorder.Body.String())
	}
}

func TestRemoteConfigTestEndpoint(t *testing.T) {
	dataDir := t.TempDir()
	db := newRemoteHandlerTestDB(t, dataDir)

	var (
		mu       sync.Mutex
		testedID int64
	)
	remoteService := service.NewRemoteConfigService(db, dataDir, func(ctx context.Context, remote model.RemoteConfig) error {
		mu.Lock()
		defer mu.Unlock()
		testedID = remote.ID
		return nil
	})

	remote, err := remoteService.CreateRemoteConfig(context.Background(), service.RemoteConfigInput{
		Name:     "ssh-test",
		Type:     "ssh",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
	}, generatePrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	router := NewRouter(
		db,
		WithJWTSecret("secret"),
		WithDataDir(dataDir),
		withRemoteConfigService(remoteService),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/remotes/"+itoa(remote.ID)+"/test", nil)
	req.Header.Set("Authorization", adminBearerToken(t, "secret"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("test status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if testedID != remote.ID {
		t.Fatalf("testedID = %d, want %d", testedID, remote.ID)
	}
}

func newRemoteHandlerTestDB(t *testing.T, dataDir string) *store.DB {
	t.Helper()

	db, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})

	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}

	return db
}

func adminBearerToken(t *testing.T, secret string) string {
	t.Helper()

	token, err := authcrypto.GenerateAccessToken(authcrypto.Claims{
		UserID: 1,
		Email:  "admin@example.com",
		Role:   "admin",
	}, secret)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	return "Bearer " + token
}

func newMultipartRemoteRequest(t *testing.T, method, path string, fields map[string]string, fileField, fileName string, fileContent []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField(%s) error = %v", key, err)
		}
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("CreateFormFile() error = %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("part.Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func generatePrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
}
