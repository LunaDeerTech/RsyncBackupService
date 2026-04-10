package handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/util"
)

const backupErrorNotFound = 40407

var downloadSplitPartPattern = regexp.MustCompile(`\.part\d+$`)

type restoreBackupRequest struct {
	RestoreType    string `json:"restore_type"`
	TargetPath     string `json:"target_path,omitempty"`
	RemoteConfigID *int64 `json:"remote_config_id,omitempty"`
	InstanceName   string `json:"instance_name"`
	Password       string `json:"password"`
	EncryptionKey  string `json:"encryption_key,omitempty"`
}

type DownloadToken struct {
	BackupID  int64
	FilePath  string
	ExpiresAt time.Time
}

type backupDownloadPart struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

type backupDownloadResponse struct {
	Mode     string               `json:"mode"`
	URL      string               `json:"url,omitempty"`
	FileName string               `json:"file_name,omitempty"`
	Parts    []backupDownloadPart `json:"parts,omitempty"`
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
		if request.RemoteConfigID != nil {
			remote, err := h.db.GetRemoteConfigByID(*request.RemoteConfigID)
			if err != nil {
				Error(w, http.StatusBadRequest, authErrorInvalidRequest, "remote_config_id: remote config not found")
				return
			}
			if remote.Type != "ssh" {
				Error(w, http.StatusBadRequest, authErrorInvalidRequest, "remote_config_id: only SSH remote configs are supported for restore")
				return
			}
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
		InstanceID:     instance.ID,
		BackupID:       &backup.ID,
		Type:           "restore",
		RestoreType:    restoreType,
		TargetPath:     targetPath,
		RemoteConfigID: request.RemoteConfigID,
		Status:         "queued",
		Progress:       0,
		CurrentStep:    "queued",
		ErrorMessage:   "",
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

	_, backup, policy, err := h.loadBackupContext(instanceID, backupID)
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

	target, err := h.db.GetBackupTargetByID(policy.TargetID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query backup target")
		return
	}

	downloadResponse, err := h.buildBackupDownloadResponse(backup, target)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, err.Error())
		return
	}

	h.writeCurrentUserAudit(r, instanceID, audit.ActionBackupDownload, map[string]any{
		"backup_id":   backup.ID,
		"policy_id":   backup.PolicyID,
		"type":        backup.Type,
		"instance_id": instanceID,
	})

	JSON(w, http.StatusOK, downloadResponse)
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

	target, err := h.loadBackupTargetForDownload(backup)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, err.Error())
		return
	}

	var streamErr error
	switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
	case "local":
		streamErr = streamLocalBackupArtifact(w, r, info.FilePath)
	case "openlist":
		streamErr = h.streamOpenListBackupArtifact(w, r, target, info.FilePath)
	default:
		streamErr = fmt.Errorf("backup download is not supported for storage type %q", target.StorageType)
	}
	if streamErr != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, streamErr.Error())
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

func (h *Handler) buildBackupDownloadResponse(backup *model.Backup, target *model.BackupTarget) (backupDownloadResponse, error) {
	if backup == nil {
		return backupDownloadResponse{}, fmt.Errorf("backup is required")
	}
	if target == nil {
		return backupDownloadResponse{}, fmt.Errorf("backup target is required")
	}
	artifactPath := strings.TrimSpace(backup.SnapshotPath)
	if artifactPath == "" {
		return backupDownloadResponse{}, fmt.Errorf("backup artifact path is required")
	}

	if basePath, ok := downloadSplitBasePath(artifactPath); ok {
		responseParts := make([]backupDownloadPart, 0, 4)
		switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
		case "local":
			parts, err := listSplitDownloadParts(basePath)
			if err != nil {
				return backupDownloadResponse{}, err
			}
			for index, partPath := range parts {
				token := h.downloadTokens.Generate(backup.ID, partPath)
				info, err := os.Stat(partPath)
				if err != nil {
					return backupDownloadResponse{}, fmt.Errorf("stat split download part %q: %w", partPath, err)
				}
				responseParts = append(responseParts, backupDownloadPart{
					Index:     index + 1,
					Name:      filepath.Base(partPath),
					URL:       "/api/v1/download/" + token,
					SizeBytes: info.Size(),
				})
			}
		case "openlist":
			parts, err := h.listOpenListSplitDownloadParts(target, basePath)
			if err != nil {
				return backupDownloadResponse{}, err
			}
			for index, part := range parts {
				responseParts = append(responseParts, backupDownloadPart{
					Index:     index + 1,
					Name:      filepath.Base(part.Path),
					URL:       "/api/v1/download/" + h.downloadTokens.Generate(backup.ID, part.Path),
					SizeBytes: part.Size,
				})
			}
		default:
			return backupDownloadResponse{}, fmt.Errorf("backup download is not supported for storage type %q", target.StorageType)
		}
		return backupDownloadResponse{
			Mode:     "split",
			FileName: filepath.Base(basePath),
			Parts:    responseParts,
		}, nil
	}

	token := h.downloadTokens.Generate(backup.ID, artifactPath)
	return backupDownloadResponse{
		Mode:     "single",
		URL:      "/api/v1/download/" + token,
		FileName: filepath.Base(artifactPath),
	}, nil
}

type openListDownloadPart struct {
	Path string
	Size int64
}

func (h *Handler) loadBackupTargetForDownload(backup *model.Backup) (*model.BackupTarget, error) {
	if h == nil || h.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	policy, err := h.db.GetPolicyByID(backup.PolicyID)
	if err != nil {
		return nil, err
	}
	return h.db.GetBackupTargetByID(policy.TargetID)
}

func (h *Handler) listOpenListSplitDownloadParts(target *model.BackupTarget, basePath string) ([]openListDownloadPart, error) {
	session, err := h.openListSessionForTarget(target)
	if err != nil {
		return nil, err
	}
	parts := make([]openListDownloadPart, 0, 4)
	for partIndex := 1; ; partIndex++ {
		partPath := fmt.Sprintf("%s.part%03d", basePath, partIndex)
		object, err := session.Get(context.Background(), partPath)
		if errors.Is(err, openlist.ErrNotFound) {
			if partIndex == 1 {
				return nil, fmt.Errorf("split download parts for %q not found", basePath)
			}
			break
		}
		if err != nil {
			return nil, err
		}
		parts = append(parts, openListDownloadPart{Path: partPath, Size: object.Size})
	}
	return parts, nil
}

func (h *Handler) openListSessionForTarget(target *model.BackupTarget) (*openlist.Session, error) {
	if h == nil || h.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if target == nil || target.RemoteConfigID == nil {
		return nil, fmt.Errorf("openlist target remote config is required")
	}
	remote, err := h.db.GetRemoteConfigByID(*target.RemoteConfigID)
	if err != nil {
		return nil, err
	}
	if !openlist.IsRemoteConfig(*remote) {
		return nil, fmt.Errorf("remote config must be openlist")
	}
	config, err := openlist.ParseConfig(*remote)
	if err != nil {
		return nil, err
	}
	return openlist.NewClient(nil).Open(context.Background(), config)
}

func (h *Handler) streamOpenListBackupArtifact(w http.ResponseWriter, r *http.Request, target *model.BackupTarget, artifactPath string) error {
	session, err := h.openListSessionForTarget(target)
	if err != nil {
		return err
	}
	resp, err := session.OpenDownload(r.Context(), artifactPath, r.Header.Get("Range"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	copyHeaderIfPresent(w.Header(), resp.Header, "Content-Type")
	copyHeaderIfPresent(w.Header(), resp.Header, "Content-Length")
	copyHeaderIfPresent(w.Header(), resp.Header, "Content-Range")
	copyHeaderIfPresent(w.Header(), resp.Header, "Accept-Ranges")
	copyHeaderIfPresent(w.Header(), resp.Header, "Last-Modified")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(artifactPath)))
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

func copyHeaderIfPresent(dst, src http.Header, key string) {
	if value := strings.TrimSpace(src.Get(key)); value != "" {
		dst.Set(key, value)
	}
}

func streamLocalBackupArtifact(w http.ResponseWriter, r *http.Request, artifactPath string) error {
	prepared, cleanup, err := prepareLocalBackupDownload(artifactPath)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	info, err := os.Stat(prepared.FilePath)
	if err != nil {
		return fmt.Errorf("stat prepared backup artifact %q: %w", prepared.FilePath, err)
	}

	file, err := os.Open(prepared.FilePath)
	if err != nil {
		return fmt.Errorf("open prepared backup artifact %q: %w", prepared.FilePath, err)
	}
	defer file.Close()
	if prepared.ContentType != "" {
		w.Header().Set("Content-Type", prepared.ContentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", prepared.FileName))
	http.ServeContent(w, r, prepared.FileName, info.ModTime(), file)
	return nil
}

type preparedBackupDownload struct {
	FilePath    string
	FileName    string
	ContentType string
}

func prepareLocalBackupDownload(artifactPath string) (*preparedBackupDownload, func(), error) {
	trimmed := strings.TrimSpace(artifactPath)
	if trimmed == "" {
		return nil, nil, fmt.Errorf("backup artifact path is required")
	}

	info, err := os.Stat(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("open backup artifact %q: %w", trimmed, err)
	}
	if info.IsDir() {
		tarballPath, cleanup, err := buildDirectoryDownload(trimmed)
		if err != nil {
			return nil, nil, err
		}
		return &preparedBackupDownload{
			FilePath:    tarballPath,
			FileName:    filepath.Base(trimmed) + ".tar.gz",
			ContentType: "application/gzip",
		}, cleanup, nil
	}

	return &preparedBackupDownload{
		FilePath: trimmed,
		FileName: filepath.Base(trimmed),
	}, nil, nil
}

func downloadSplitBasePath(artifactPath string) (string, bool) {
	trimmed := strings.TrimSpace(artifactPath)
	if !downloadSplitPartPattern.MatchString(trimmed) {
		return "", false
	}
	return strings.TrimSuffix(trimmed, filepath.Ext(trimmed)), true
}

func listSplitDownloadParts(basePath string) ([]string, error) {
	parts, err := filepath.Glob(basePath + ".part*")
	if err != nil {
		return nil, fmt.Errorf("glob split download parts for %q: %w", basePath, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("split download parts for %q not found", basePath)
	}
	parts = filterExistingDownloadParts(parts)
	if len(parts) == 0 {
		return nil, fmt.Errorf("split download parts for %q not found", basePath)
	}
	sort.Strings(parts)
	return parts, nil
}

func filterExistingDownloadParts(parts []string) []string {
	filtered := make([]string, 0, len(parts))
	for _, partPath := range parts {
		if downloadSplitPartPattern.MatchString(partPath) {
			filtered = append(filtered, partPath)
		}
	}
	return filtered
}

func buildDirectoryDownload(root string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "rbs-download-dir-*.tar.gz")
	if err != nil {
		return "", nil, fmt.Errorf("create directory download temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if err := streamDirectoryAsTarGz(tempFile, root); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("package backup directory %q: %w", root, err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("close directory download temp file %q: %w", tempPath, err)
	}
	return tempPath, func() {
		_ = os.Remove(tempPath)
	}, nil
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
