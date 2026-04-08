package handler

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/util"
)

const backupErrorNotFound = 40407

type restoreBackupRequest struct {
	RestoreType   string `json:"restore_type"`
	TargetPath    string `json:"target_path,omitempty"`
	InstanceName  string `json:"instance_name"`
	Password      string `json:"password"`
	EncryptionKey string `json:"encryption_key,omitempty"`
}

type DownloadToken struct {
	BackupID  int64
	FilePath  string
	ExpiresAt time.Time
}

type DownloadTokenManager struct {
	mu     sync.Mutex
	tokens map[string]*DownloadToken
	now    func() time.Time
	ttl    time.Duration
}

func NewDownloadTokenManager() *DownloadTokenManager {
	return &DownloadTokenManager{
		tokens: make(map[string]*DownloadToken),
		now:    time.Now,
		ttl:    5 * time.Minute,
	}
}

func (m *DownloadTokenManager) Generate(backupID int64, filePath string) string {
	if m == nil {
		return ""
	}
	if m.now == nil {
		m.now = time.Now
	}
	if m.ttl <= 0 {
		m.ttl = 5 * time.Minute
	}
	token := randomToken(32)
	m.mu.Lock()
	m.tokens[token] = &DownloadToken{
		BackupID:  backupID,
		FilePath:  strings.TrimSpace(filePath),
		ExpiresAt: m.now().UTC().Add(m.ttl),
	}
	m.mu.Unlock()
	return token
}

func (m *DownloadTokenManager) Validate(token string) (*DownloadToken, error) {
	if m == nil {
		return nil, fmt.Errorf("download token manager is unavailable")
	}
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, fmt.Errorf("download token is required")
	}
	if m.now == nil {
		m.now = time.Now
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.tokens[trimmed]
	if !ok {
		return nil, fmt.Errorf("download token is invalid")
	}
	if m.now().UTC().After(info.ExpiresAt) {
		delete(m.tokens, trimmed)
		return nil, fmt.Errorf("download token has expired")
	}
	copyInfo := *info
	return &copyInfo, nil
}

func (m *DownloadTokenManager) Revoke(token string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.tokens, strings.TrimSpace(token))
	m.mu.Unlock()
}

func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	pagination := ParsePagination(r)
	total, err := h.db.CountBackupsByInstance(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count backups")
		return
	}

	backups, err := h.db.ListBackupsByInstance(instanceID, pagination.PageSize, (pagination.Page-1)*pagination.PageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list backups")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      backups,
		Total:      int64(total),
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(int64(total), pagination.PageSize),
	})
}

func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	if h.taskQueue == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "task queue unavailable")
		return
	}

	instanceID, backupID, err := backupRequestIDs(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	instance, backup, policy, err := h.loadBackupContext(instanceID, backupID)
	if err != nil {
		writeBackupError(w, err, "failed to query backup")
		return
	}

	var request restoreBackupRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	if strings.TrimSpace(request.InstanceName) != instance.Name {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "instance_name does not match the target instance")
		return
	}
	claims := middleware.MustGetUser(r.Context())
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}
	if !authcrypto.CheckPassword(strings.TrimSpace(request.Password), user.PasswordHash) {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "password is incorrect")
		return
	}
	if backup.Status != "success" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "backup must be in success status before restore")
		return
	}

	restoreType := strings.ToLower(strings.TrimSpace(request.RestoreType))
	targetPath := strings.TrimSpace(request.TargetPath)
	switch restoreType {
	case "source":
		targetPath = ""
	case "custom":
		if targetPath == "" {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "target_path is required when restore_type is custom")
			return
		}
		if err := util.ValidatePath(targetPath); err != nil {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "target_path: "+err.Error())
			return
		}
	default:
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "restore_type must be source or custom")
		return
	}

	encryptionKey := strings.TrimSpace(request.EncryptionKey)
	if backup.Type == "cold" && policy.Encryption {
		if encryptionKey == "" {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "encryption_key is required for encrypted cold backup restore")
			return
		}
		if policy.EncryptionKeyHash != nil && *policy.EncryptionKeyHash != "" && !authcrypto.ValidateEncryptionKey(encryptionKey, *policy.EncryptionKeyHash) {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "encryption_key does not match policy")
			return
		}
	}

	task := &model.Task{
		InstanceID:   instance.ID,
		BackupID:     &backup.ID,
		Type:         "restore",
		RestoreType:  restoreType,
		TargetPath:   targetPath,
		Status:       "queued",
		Progress:     0,
		CurrentStep:  "queued",
		ErrorMessage: "",
	}
	if err := h.db.CreateTask(task); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to create restore task")
		return
	}
	if encryptionKey != "" {
		h.taskQueue.SetRestoreEncryptionKey(task.ID, encryptionKey)
	}
	if err := h.taskQueue.Enqueue(task); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to enqueue restore task")
		return
	}
	h.writeCurrentUserAudit(r, instance.ID, audit.ActionRestoreTrigger, map[string]any{
		"task_id":      task.ID,
		"backup_id":    backup.ID,
		"policy_id":    policy.ID,
		"backup_type":  backup.Type,
		"restore_type": restoreType,
		"target_path":  targetPath,
	})

	JSON(w, http.StatusCreated, map[string]any{"task": task})
}

func (h *Handler) GenerateBackupDownloadURL(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	if h.downloadTokens == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "download token manager unavailable")
		return
	}

	instanceID, backupID, err := backupRequestIDs(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	_, backup, _, err := h.loadBackupContext(instanceID, backupID)
	if err != nil {
		writeBackupError(w, err, "failed to query backup")
		return
	}
	if backup.Type != "cold" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only cold backups can be downloaded")
		return
	}
	if backup.Status != "success" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "backup must be in success status before download")
		return
	}

	token := h.downloadTokens.Generate(backup.ID, backup.SnapshotPath)

	h.writeCurrentUserAudit(r, instanceID, audit.ActionBackupDownload, map[string]any{
		"backup_id":   backup.ID,
		"policy_id":   backup.PolicyID,
		"type":        backup.Type,
		"instance_id": instanceID,
	})

	JSON(w, http.StatusOK, map[string]any{
		"url": "/api/v1/download/" + token,
	})
}

func (h *Handler) DownloadBackupByToken(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	if h.downloadTokens == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "download token manager unavailable")
		return
	}

	token := strings.TrimSpace(r.PathValue("token"))
	info, err := h.downloadTokens.Validate(token)
	if err != nil {
		Error(w, http.StatusForbidden, 40302, err.Error())
		return
	}

	backup, err := h.db.GetBackupByID(info.BackupID)
	if err != nil {
		writeBackupError(w, err, "failed to query backup")
		return
	}
	if backup.Type != "cold" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only cold backups can be downloaded")
		return
	}

	h.downloadTokens.Revoke(token)
	if err := streamLocalBackupArtifact(w, r, info.FilePath); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, err.Error())
	}
}

func (h *Handler) loadBackupContext(instanceID, backupID int64) (*model.Instance, *model.Backup, *model.Policy, error) {
	instance, err := h.db.GetInstanceByID(instanceID)
	if err != nil {
		return nil, nil, nil, err
	}
	backup, err := h.db.GetBackupByID(backupID)
	if err != nil {
		return nil, nil, nil, err
	}
	if backup.InstanceID != instanceID {
		return nil, nil, nil, sql.ErrNoRows
	}
	policy, err := h.db.GetPolicyByID(backup.PolicyID)
	if err != nil {
		return nil, nil, nil, err
	}
	return instance, backup, policy, nil
}

func writeBackupError(w http.ResponseWriter, err error, defaultMessage string) {
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, backupErrorNotFound, "backup not found")
		return
	}
	Error(w, http.StatusInternalServerError, authErrorInternal, defaultMessage)
}

func backupRequestIDs(r *http.Request) (int64, int64, error) {
	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		return 0, 0, err
	}
	rawBackupID := strings.TrimSpace(r.PathValue("bid"))
	if rawBackupID == "" {
		return 0, 0, fmt.Errorf("backup id is required")
	}
	backupID, err := strconv.ParseInt(rawBackupID, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse backup id %q: %w", rawBackupID, err)
	}
	if backupID <= 0 {
		return 0, 0, fmt.Errorf("backup id must be positive")
	}
	return instanceID, backupID, nil
}

func streamLocalBackupArtifact(w http.ResponseWriter, r *http.Request, artifactPath string) error {
	info, err := os.Stat(artifactPath)
	if err != nil {
		return fmt.Errorf("open backup artifact %q: %w", artifactPath, err)
	}
	if info.IsDir() {
		fileName := filepath.Base(artifactPath) + ".tar.gz"
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
		return streamDirectoryAsTarGz(w, artifactPath)
	}

	file, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("open backup artifact %q: %w", artifactPath, err)
	}
	defer file.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(artifactPath)))
	http.ServeContent(w, r, filepath.Base(artifactPath), info.ModTime(), file)
	return nil
}

func streamDirectoryAsTarGz(w io.Writer, root string) error {
	gzipWriter := gzip.NewWriter(w)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(root, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarWriter, file)
		return err
	})
}

func randomToken(size int) string {
	if size <= 0 {
		size = 16
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(raw)
}
