package openlist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"rsync-backup-service/internal/model"
)

func TestSessionSupportsFileLifecycle(t *testing.T) {
	token := "token-123"
	createdDirs := make([]string, 0, 2)
	removedNames := make([]string, 0, 1)
	uploadedPaths := make([]string, 0, 1)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"token": token,
				},
			})
		case r.URL.Path == "/api/fs/get":
			assertAuth(t, r, token)
			var payload map[string]string
			decodeJSONBody(t, r, &payload)
			if payload["path"] != "/" && !strings.HasSuffix(payload["path"], ".tar") && !containsString(createdDirs, payload["path"]) {
				writeJSON(t, w, http.StatusOK, map[string]any{"code": 500, "message": "object not found", "data": nil})
				return
			}
			isDir := payload["path"] == "/" || !strings.HasSuffix(payload["path"], ".tar")
			writeJSON(t, w, http.StatusOK, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"path":          payload["path"],
					"name":          path.Base(payload["path"]),
					"size":          42,
					"is_dir":        isDir,
					"sign":          "sig-1",
					"mount_details": map[string]any{"driver_name": "Local", "total_space": 1000, "free_space": 500},
				},
			})
		case r.URL.Path == "/api/fs/mkdir":
			assertAuth(t, r, token)
			var payload map[string]string
			decodeJSONBody(t, r, &payload)
			createdDirs = append(createdDirs, payload["path"])
			writeJSON(t, w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": nil})
		case r.URL.Path == "/api/fs/remove":
			assertAuth(t, r, token)
			var payload struct {
				Dir   string   `json:"dir"`
				Names []string `json:"names"`
			}
			decodeJSONBody(t, r, &payload)
			if payload.Dir != "/backups/mysql/20260410" {
				t.Fatalf("remove dir = %q, want %q", payload.Dir, "/backups/mysql/20260410")
			}
			removedNames = append(removedNames, payload.Names...)
			writeJSON(t, w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": nil})
		case r.URL.Path == "/api/fs/put":
			assertAuth(t, r, token)
			uploadedPaths = append(uploadedPaths, r.Header.Get("File-Path"))
			content, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(upload body) error = %v", err)
			}
			if string(content) != "payload-data" {
				t.Fatalf("upload body = %q, want %q", string(content), "payload-data")
			}
			writeJSON(t, w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": nil})
		case strings.HasPrefix(r.URL.Path, "/@file/link/path/"):
			assertAuth(t, r, token)
			writeJSON(t, w, http.StatusOK, map[string]any{"data": server.URL + "/downloads/artifact.tar"})
		case r.URL.Path == "/downloads/artifact.tar":
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("downloaded-data"))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	session, err := client.Open(context.Background(), Config{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	object, err := session.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("Get(/) error = %v", err)
	}
	if !object.IsDir {
		t.Fatal("Get(/).IsDir = false, want true")
	}

	if err := session.EnsureDir(context.Background(), "/backups/mysql/20260410"); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	if len(createdDirs) != 3 {
		t.Fatalf("createdDirs = %v, want 3 mkdir calls", createdDirs)
	}

	localFile := filepath.Join(t.TempDir(), "artifact.tar")
	if err := os.WriteFile(localFile, []byte("payload-data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := session.UploadFile(context.Background(), localFile, "/backups/mysql/20260410/artifact.tar"); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if len(uploadedPaths) != 1 || uploadedPaths[0] != "/backups/mysql/20260410/artifact.tar" {
		t.Fatalf("uploadedPaths = %v", uploadedPaths)
	}

	resp, err := session.OpenDownload(context.Background(), "/backups/mysql/20260410/artifact.tar", "")
	if err != nil {
		t.Fatalf("OpenDownload() error = %v", err)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(download body) error = %v", err)
	}
	if string(content) != "downloaded-data" {
		t.Fatalf("download content = %q, want %q", string(content), "downloaded-data")
	}

	if err := session.RemovePath(context.Background(), "/backups/mysql/20260410/artifact.tar"); err != nil {
		t.Fatalf("RemovePath() error = %v", err)
	}
	if len(removedNames) != 1 || removedNames[0] != "artifact.tar" {
		t.Fatalf("removedNames = %v", removedNames)
	}
}

func TestParseConfigSupportsOpenListRemote(t *testing.T) {
	encoded, err := EncodeStoredConfig("secret", "")
	if err != nil {
		t.Fatalf("EncodeStoredConfig() error = %v", err)
	}
	config, err := ParseConfig(model.RemoteConfig{
		ID:            7,
		Type:          "openlist",
		Host:          "https://openlist.example.com",
		Username:      "admin",
		CloudProvider: stringPtr("openlist"),
		CloudConfig:   encoded,
	})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.BaseURL != "https://openlist.example.com" || config.Username != "admin" || config.Password != "secret" {
		t.Fatalf("ParseConfig() = %+v", config)
	}
}

func TestSessionEnsureDirSkipsExistingParentDirectories(t *testing.T) {
	token := "token-123"
	createdDirs := make([]string, 0, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"token": token,
				},
			})
		case "/api/fs/get":
			assertAuth(t, r, token)
			var payload map[string]string
			decodeJSONBody(t, r, &payload)
			switch payload["path"] {
			case "/mount", "/mount/existing":
				writeJSON(t, w, http.StatusOK, map[string]any{
					"code":    200,
					"message": "success",
					"data": map[string]any{
						"path":   payload["path"],
						"name":   path.Base(payload["path"]),
						"is_dir": true,
					},
				})
			case "/mount/existing/new-dir":
				writeJSON(t, w, http.StatusOK, map[string]any{"code": 500, "message": "object not found", "data": nil})
			default:
				t.Fatalf("unexpected get path: %s", payload["path"])
			}
		case "/api/fs/mkdir":
			assertAuth(t, r, token)
			var payload map[string]string
			decodeJSONBody(t, r, &payload)
			createdDirs = append(createdDirs, payload["path"])
			if payload["path"] != "/mount/existing/new-dir" {
				t.Fatalf("mkdir path = %q, want %q", payload["path"], "/mount/existing/new-dir")
			}
			writeJSON(t, w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": nil})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	session, err := NewClient(server.Client()).Open(context.Background(), Config{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := session.EnsureDir(context.Background(), "/mount/existing/new-dir"); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	if len(createdDirs) != 1 {
		t.Fatalf("createdDirs = %v, want one mkdir call", createdDirs)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

func decodeJSONBody(t *testing.T, r *http.Request, out any) {
	t.Helper()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		t.Fatalf("json.NewDecoder().Decode() error = %v", err)
	}
}

func assertAuth(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
}

func stringPtr(value string) *string {
	return &value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
