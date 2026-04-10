package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/util"
)

const (
	remoteErrorNotFound = 40402
	remoteErrorConflict = 40902
	maxRemoteFormMemory = 8 << 20
)

type remoteConfigResponse struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	Username      string    `json:"username"`
	CloudProvider *string   `json:"cloud_provider,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type remoteConfigUpdateFields struct {
	hasName          bool
	name             string
	hasType          bool
	remoteType       string
	hasHost          bool
	host             string
	hasPort          bool
	port             int
	hasUsername      bool
	username         string
	hasPassword      bool
	password         string
	hasCloudProvider bool
	cloudProvider    *string
	hasCloudConfig   bool
	cloudConfig      *string
	privateKeyPEM    []byte
	hasPrivateKey    bool
}

func (h *Handler) ListRemoteConfigs(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	pagination := ParsePagination(r)
	total, err := h.db.CountRemoteConfigs()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count remote configs")
		return
	}

	remotes, err := h.db.ListRemoteConfigsPage(pagination.PageSize, (pagination.Page-1)*pagination.PageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list remote configs")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      toRemoteConfigResponses(remotes),
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(total, pagination.PageSize),
	})
}

func (h *Handler) CreateRemoteConfig(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.remoteConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "remote config service unavailable")
		return
	}

	input, privateKeyPEM, err := parseCreateRemoteConfigForm(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := validateRemoteConfigInput(input, true, len(privateKeyPEM) > 0); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureRemoteConfigNameAvailable(input.Name, 0); err != nil {
		writeRemoteConfigError(w, err, "failed to query remote config")
		return
	}

	remote, err := h.remoteConfigs.CreateRemoteConfig(r.Context(), input, privateKeyPEM)
	if err != nil {
		writeRemoteConfigError(w, err, "failed to create remote config")
		return
	}

	JSON(w, http.StatusCreated, toRemoteConfigResponse(*remote))
}

func (h *Handler) UpdateRemoteConfig(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.remoteConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "remote config service unavailable")
		return
	}

	remoteID, err := remoteConfigIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid remote config id")
		return
	}

	current, err := h.db.GetRemoteConfigByID(remoteID)
	if err != nil {
		writeRemoteConfigError(w, err, "failed to query remote config")
		return
	}

	fields, err := parseUpdateRemoteConfigForm(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	input := service.RemoteConfigInput{
		Name:          current.Name,
		Type:          current.Type,
		Host:          current.Host,
		Port:          current.Port,
		Username:      current.Username,
		CloudProvider: cloneOptionalString(current.CloudProvider),
		CloudConfig:   cloneOptionalString(current.CloudConfig),
	}

	if fields.hasName {
		input.Name = strings.TrimSpace(fields.name)
	}
	if fields.hasType {
		input.Type = normalizeRemoteType(fields.remoteType)
	}
	if fields.hasHost {
		input.Host = strings.TrimSpace(fields.host)
	}
	if fields.hasPort {
		input.Port = fields.port
	}
	if fields.hasUsername {
		input.Username = strings.TrimSpace(fields.username)
	}
	if fields.hasPassword {
		input.CloudConfig = nil
	}
	if fields.hasCloudProvider {
		input.CloudProvider = cloneOptionalString(fields.cloudProvider)
	}
	if fields.hasCloudConfig {
		input.CloudConfig = cloneOptionalString(fields.cloudConfig)
	}

	if input.Type == "cloud" {
		input.Host = ""
		input.Port = 0
		input.Username = ""
	}
	if input.Type == "openlist" {
		input.Port = 0
		input.CloudProvider = cloneOptionalString(stringPtr("openlist"))
		cloudConfig, err := buildOpenListCloudConfig(input.Host, current.CloudConfig, fields.password, fields.hasPassword)
		if err != nil {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
			return
		}
		input.CloudConfig = cloudConfig
	}

	requirePrivateKey := current.PrivateKeyPath == "" || current.Type != "ssh" || input.Type != current.Type
	if err := validateRemoteConfigInput(input, requirePrivateKey, fields.hasPrivateKey); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureRemoteConfigNameAvailable(input.Name, current.ID); err != nil {
		writeRemoteConfigError(w, err, "failed to query remote config")
		return
	}

	updated, err := h.remoteConfigs.UpdateRemoteConfig(r.Context(), remoteID, input, fields.privateKeyPEM, fields.hasPrivateKey)
	if err != nil {
		writeRemoteConfigError(w, err, "failed to update remote config")
		return
	}

	JSON(w, http.StatusOK, toRemoteConfigResponse(*updated))
}

func (h *Handler) DeleteRemoteConfig(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.remoteConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "remote config service unavailable")
		return
	}

	remoteID, err := remoteConfigIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid remote config id")
		return
	}

	if err := h.remoteConfigs.DeleteRemoteConfig(r.Context(), remoteID); err != nil {
		writeRemoteConfigError(w, err, "failed to delete remote config")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "remote config deleted"})
}

func (h *Handler) TestRemoteConfig(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.remoteConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "remote config service unavailable")
		return
	}

	remoteID, err := remoteConfigIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid remote config id")
		return
	}

	if err := h.remoteConfigs.TestRemoteConfigConnection(r.Context(), remoteID); err != nil {
		writeRemoteConfigError(w, err, "ssh connection test failed")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "ssh connection test succeeded"})
}

func parseCreateRemoteConfigForm(r *http.Request) (service.RemoteConfigInput, []byte, error) {
	if err := r.ParseMultipartForm(maxRemoteFormMemory); err != nil {
		return service.RemoteConfigInput{}, nil, fmt.Errorf("invalid multipart form")
	}

	input := service.RemoteConfigInput{
		Name:          strings.TrimSpace(formFirstValue(r.MultipartForm, "name")),
		Type:          normalizeRemoteType(formFirstValue(r.MultipartForm, "type")),
		Host:          strings.TrimSpace(formFirstValue(r.MultipartForm, "host")),
		Username:      strings.TrimSpace(formFirstValue(r.MultipartForm, "username")),
		CloudProvider: optionalFormString(r.MultipartForm, "cloud_provider"),
		CloudConfig:   optionalFormString(r.MultipartForm, "cloud_config"),
	}
	password := strings.TrimSpace(formFirstValue(r.MultipartForm, "password"))

	port, err := parsePortValue(formFirstValue(r.MultipartForm, "port"), input.Type != "ssh")
	if err != nil {
		return service.RemoteConfigInput{}, nil, err
	}
	input.Port = port

	privateKeyPEM, _, err := readFormFile(r.MultipartForm, "private_key")
	if err != nil {
		return service.RemoteConfigInput{}, nil, err
	}

	if input.Type == "cloud" {
		input.Host = ""
		input.Port = 0
		input.Username = ""
		if len(privateKeyPEM) > 0 {
			return service.RemoteConfigInput{}, nil, service.ErrPrivateKeyNotSupported
		}
	}
	if input.Type == "openlist" {
		input.Port = 0
		input.CloudProvider = cloneOptionalString(stringPtr("openlist"))
		cloudConfig, err := buildOpenListCloudConfig(input.Host, nil, password, true)
		if err != nil {
			return service.RemoteConfigInput{}, nil, err
		}
		input.CloudConfig = cloudConfig
		if len(privateKeyPEM) > 0 {
			return service.RemoteConfigInput{}, nil, service.ErrPrivateKeyNotSupported
		}
	}

	return input, privateKeyPEM, nil
}

func parseUpdateRemoteConfigForm(r *http.Request) (remoteConfigUpdateFields, error) {
	if err := r.ParseMultipartForm(maxRemoteFormMemory); err != nil {
		return remoteConfigUpdateFields{}, fmt.Errorf("invalid multipart form")
	}

	fields := remoteConfigUpdateFields{}
	if value, ok := formValue(r.MultipartForm, "name"); ok {
		fields.hasName = true
		fields.name = value
	}
	if value, ok := formValue(r.MultipartForm, "type"); ok {
		fields.hasType = true
		fields.remoteType = value
	}
	if value, ok := formValue(r.MultipartForm, "host"); ok {
		fields.hasHost = true
		fields.host = value
	}
	if value, ok := formValue(r.MultipartForm, "port"); ok {
		port, err := parsePortValue(value, true)
		if err != nil {
			return remoteConfigUpdateFields{}, err
		}
		fields.hasPort = true
		fields.port = port
	}
	if value, ok := formValue(r.MultipartForm, "username"); ok {
		fields.hasUsername = true
		fields.username = value
	}
	if value, ok := formValue(r.MultipartForm, "password"); ok {
		fields.hasPassword = true
		fields.password = value
	}
	if _, ok := r.MultipartForm.Value["cloud_provider"]; ok {
		fields.hasCloudProvider = true
		fields.cloudProvider = optionalFormString(r.MultipartForm, "cloud_provider")
	}
	if _, ok := r.MultipartForm.Value["cloud_config"]; ok {
		fields.hasCloudConfig = true
		fields.cloudConfig = optionalFormString(r.MultipartForm, "cloud_config")
	}

	privateKeyPEM, hasPrivateKey, err := readFormFile(r.MultipartForm, "private_key")
	if err != nil {
		return remoteConfigUpdateFields{}, err
	}
	fields.privateKeyPEM = privateKeyPEM
	fields.hasPrivateKey = hasPrivateKey

	return fields, nil
}

func validateRemoteConfigInput(input service.RemoteConfigInput, requirePrivateKey bool, hasPrivateKey bool) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("name is required")
	}

	switch input.Type {
	case "ssh":
		if strings.TrimSpace(input.Host) == "" {
			return fmt.Errorf("host is required")
		}
		if err := util.ValidateSSHHost(input.Host); err != nil {
			return fmt.Errorf("host: %w", err)
		}
		if input.Port < 1 || input.Port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		if strings.TrimSpace(input.Username) == "" {
			return fmt.Errorf("username is required")
		}
		if requirePrivateKey && !hasPrivateKey {
			return service.ErrPrivateKeyRequired
		}
	case "openlist":
		if hasPrivateKey {
			return service.ErrPrivateKeyNotSupported
		}
		if strings.TrimSpace(input.Host) == "" {
			return fmt.Errorf("host is required")
		}
		parsedURL, err := url.Parse(strings.TrimSpace(input.Host))
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("host must be a valid OpenList base URL")
		}
		if strings.TrimSpace(input.Username) == "" {
			return fmt.Errorf("username is required")
		}
		if input.CloudConfig == nil {
			return fmt.Errorf("password is required")
		}
		_, err = openlist.ParseConfig(model.RemoteConfig{
			Type:          "openlist",
			Host:          input.Host,
			Username:      input.Username,
			CloudProvider: cloneOptionalString(stringPtr("openlist")),
			CloudConfig:   cloneOptionalString(input.CloudConfig),
		})
		if err != nil {
			return err
		}
	case "cloud":
		if hasPrivateKey {
			return service.ErrPrivateKeyNotSupported
		}
	default:
		return fmt.Errorf("type must be ssh, openlist, or cloud")
	}

	return nil
}

func (h *Handler) ensureRemoteConfigNameAvailable(name string, currentID int64) error {
	remote, err := h.db.GetRemoteConfigByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if remote.ID != currentID {
		return fmt.Errorf("%w", errRemoteConfigNameExists)
	}

	return nil
}

var errRemoteConfigNameExists = errors.New("remote config name already exists")

func writeRemoteConfigError(w http.ResponseWriter, err error, defaultMessage string) {
	var inUseErr *service.RemoteConfigInUseError
	switch {
	case errors.As(err, &inUseErr):
		ErrorWithData(w, http.StatusBadRequest, authErrorInvalidRequest, inUseErr.Error(), inUseErr.Usage)
	case errors.Is(err, errRemoteConfigNameExists):
		Error(w, http.StatusConflict, remoteErrorConflict, "remote config name already exists")
	case errors.Is(err, sql.ErrNoRows):
		Error(w, http.StatusNotFound, remoteErrorNotFound, "remote config not found")
	case errors.Is(err, service.ErrInvalidPrivateKey):
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid private key")
	case errors.Is(err, service.ErrPrivateKeyRequired):
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "private key is required")
	case errors.Is(err, service.ErrPrivateKeyNotSupported):
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "private key is only supported for ssh remotes")
	case errors.Is(err, service.ErrSSHTestNotSupported):
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only ssh remotes support connection testing")
	case errors.Is(err, service.ErrRemoteTestNotSupported):
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only ssh or OpenList remotes support connection testing")
	case errors.Is(err, service.ErrRemoteConfigUnavailable):
		Error(w, http.StatusInternalServerError, authErrorInternal, "remote config service unavailable")
	default:
		message := strings.TrimSpace(err.Error())
		switch {
		case strings.HasPrefix(message, "ssh "), strings.HasPrefix(message, "private key file "):
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, message)
		default:
			Error(w, http.StatusInternalServerError, authErrorInternal, defaultMessage)
		}
	}
}

func remoteConfigIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("remote config id is required")
	}

	remoteID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse remote config id %q: %w", rawID, err)
	}
	if remoteID <= 0 {
		return 0, fmt.Errorf("remote config id must be positive")
	}

	return remoteID, nil
}

func parsePortValue(raw string, allowEmpty bool) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" && allowEmpty {
		return 0, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("port must be a valid integer")
	}

	return port, nil
}

func formFirstValue(form *multipart.Form, key string) string {
	value, _ := formValue(form, key)
	return value
}

func formValue(form *multipart.Form, key string) (string, bool) {
	if form == nil || form.Value == nil {
		return "", false
	}

	values, ok := form.Value[key]
	if !ok || len(values) == 0 {
		return "", false
	}

	return values[0], true
}

func optionalFormString(form *multipart.Form, key string) *string {
	value, ok := formValue(form, key)
	if !ok {
		return nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &value
}

func readFormFile(form *multipart.Form, key string) ([]byte, bool, error) {
	if form == nil || form.File == nil {
		return nil, false, nil
	}

	files, ok := form.File[key]
	if !ok || len(files) == 0 {
		return nil, false, nil
	}

	file, err := files[0].Open()
	if err != nil {
		return nil, false, fmt.Errorf("failed to read uploaded private key")
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, false, fmt.Errorf("failed to read uploaded private key")
	}
	if len(content) > 1<<20 {
		return nil, false, fmt.Errorf("uploaded private key is too large")
	}

	return content, true, nil
}

func normalizeRemoteType(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func buildOpenListCloudConfig(baseURL string, currentConfig *string, password string, hasPassword bool) (*string, error) {
	stored, err := openlist.DecodeStoredConfig(currentConfig)
	if err != nil {
		return nil, fmt.Errorf("password is invalid")
	}
	resolvedPassword := stored.Password
	if hasPassword {
		resolvedPassword = strings.TrimSpace(password)
	}
	if resolvedPassword == "" {
		return nil, fmt.Errorf("password is required")
	}
	return openlist.EncodeStoredConfig(resolvedPassword, strings.TrimSpace(baseURL))
}

func toRemoteConfigResponse(remote model.RemoteConfig) remoteConfigResponse {
	return remoteConfigResponse{
		ID:            remote.ID,
		Name:          remote.Name,
		Type:          remote.Type,
		Host:          remote.Host,
		Port:          remote.Port,
		Username:      remote.Username,
		CloudProvider: cloneOptionalString(remote.CloudProvider),
		CreatedAt:     remote.CreatedAt,
		UpdatedAt:     remote.UpdatedAt,
	}
}

func toRemoteConfigResponses(remotes []model.RemoteConfig) []remoteConfigResponse {
	responses := make([]remoteConfigResponse, 0, len(remotes))
	for _, remote := range remotes {
		responses = append(responses, toRemoteConfigResponse(remote))
	}

	return responses
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func stringPtr(value string) *string {
	return &value
}
