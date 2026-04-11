package openlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"strings"
	"time"

	"rsync-backup-service/internal/model"
)

var ErrNotFound = errors.New("openlist object not found")

type Config struct {
	BaseURL  string
	Username string
	Password string
}

type StoredConfig struct {
	Password string `json:"password,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

type StorageDetails struct {
	DriverName string `json:"driver_name"`
	TotalSpace int64  `json:"total_space"`
	FreeSpace  int64  `json:"free_space"`
}

type FsObject struct {
	Path         string          `json:"path"`
	Name         string          `json:"name"`
	Size         int64           `json:"size"`
	IsDir        bool            `json:"is_dir"`
	Sign         string          `json:"sign"`
	MountDetails *StorageDetails `json:"mount_details"`
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type Client struct {
	httpClient *http.Client
}

type Session struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{httpClient: httpClient}
}

func IsRemoteConfig(remote model.RemoteConfig) bool {
	remoteType := strings.ToLower(strings.TrimSpace(remote.Type))
	if remoteType == "openlist" {
		return true
	}
	return remoteType == "cloud" && strings.EqualFold(optionalString(remote.CloudProvider), "openlist")
}

func EncodeStoredConfig(password, baseURL string) (*string, error) {
	payload, err := json.Marshal(StoredConfig{
		Password: strings.TrimSpace(password),
		BaseURL:  strings.TrimSpace(baseURL),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal openlist config: %w", err)
	}
	encoded := string(payload)
	return &encoded, nil
}

func DecodeStoredConfig(raw *string) (StoredConfig, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return StoredConfig{}, nil
	}
	var decoded StoredConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(*raw)), &decoded); err != nil {
		return StoredConfig{}, fmt.Errorf("decode openlist config: %w", err)
	}
	decoded.Password = strings.TrimSpace(decoded.Password)
	decoded.BaseURL = strings.TrimSpace(decoded.BaseURL)
	return decoded, nil
}

func ParseConfig(remote model.RemoteConfig) (Config, error) {
	if !IsRemoteConfig(remote) {
		return Config{}, fmt.Errorf("remote config %d is not an openlist config", remote.ID)
	}
	stored, err := DecodeStoredConfig(remote.CloudConfig)
	if err != nil {
		return Config{}, err
	}
	baseURL := strings.TrimSpace(remote.Host)
	if baseURL == "" {
		baseURL = stored.BaseURL
	}
	config := Config{
		BaseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Username: strings.TrimSpace(remote.Username),
		Password: strings.TrimSpace(stored.Password),
	}
	if config.BaseURL == "" {
		return Config{}, fmt.Errorf("openlist base url is required")
	}
	parsedURL, err := url.Parse(config.BaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Config{}, fmt.Errorf("openlist base url is invalid")
	}
	if config.Username == "" {
		return Config{}, fmt.Errorf("openlist username is required")
	}
	if config.Password == "" {
		return Config{}, fmt.Errorf("openlist password is required")
	}
	return config, nil
}

func VerifyConnection(ctx context.Context, remote model.RemoteConfig) error {
	config, err := ParseConfig(remote)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	session, err := NewClient(nil).Open(ctx, config)
	if err != nil {
		return err
	}
	_, err = session.Get(ctx, "/")
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) Open(ctx context.Context, config Config) (*Session, error) {
	if c == nil {
		c = NewClient(nil)
	}
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" || strings.TrimSpace(config.Username) == "" || strings.TrimSpace(config.Password) == "" {
		return nil, fmt.Errorf("openlist credentials are incomplete")
	}
	requestBody, err := json.Marshal(map[string]string{
		"username": strings.TrimSpace(config.Username),
		"password": strings.TrimSpace(config.Password),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal openlist login request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.BaseURL+"/api/auth/login", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build openlist login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openlist login request failed: %w", err)
	}
	defer resp.Body.Close()

	var envelope apiEnvelope
	if err := decodeEnvelope(resp, &envelope); err != nil {
		return nil, fmt.Errorf("openlist login failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || envelope.Code != http.StatusOK {
		return nil, fmt.Errorf("openlist login failed: %s", responseMessage(resp.Status, envelope.Message))
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, fmt.Errorf("decode openlist login response: %w", err)
	}
	if strings.TrimSpace(payload.Token) == "" {
		return nil, fmt.Errorf("openlist login failed: token is missing")
	}

	return &Session{
		httpClient: c.httpClient,
		baseURL:    config.BaseURL,
		token:      strings.TrimSpace(payload.Token),
	}, nil
}

func (s *Session) Get(ctx context.Context, remotePath string) (*FsObject, error) {
	var object FsObject
	if err := s.postJSON(ctx, "/api/fs/get", map[string]string{"path": NormalizePath(remotePath)}, &object); err != nil {
		return nil, err
	}
	return &object, nil
}

func (s *Session) Mkdir(ctx context.Context, remotePath string) error {
	return s.postJSON(ctx, "/api/fs/mkdir", map[string]string{"path": NormalizePath(remotePath)}, nil)
}

func (s *Session) EnsureDir(ctx context.Context, remotePath string) error {
	cleaned := NormalizePath(remotePath)
	if cleaned == "/" {
		return nil
	}
	segments := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	current := "/"
	for _, segment := range segments {
		current = pathpkg.Join(current, segment)
		object, err := s.Get(ctx, current)
		if err == nil {
			if !object.IsDir {
				return fmt.Errorf("openlist path %q exists and is not a directory", current)
			}
			continue
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if err := s.Mkdir(ctx, current); err != nil && !isAlreadyExistsError(err) {
			return err
		}
	}
	return nil
}

func (s *Session) Remove(ctx context.Context, dirPath string, names []string) error {
	payload := map[string]any{
		"dir":   NormalizePath(dirPath),
		"names": names,
	}
	return s.postJSON(ctx, "/api/fs/remove", payload, nil)
}

func (s *Session) RemovePath(ctx context.Context, remotePath string) error {
	cleaned := NormalizePath(remotePath)
	if cleaned == "/" {
		return fmt.Errorf("openlist root path cannot be removed")
	}
	return s.Remove(ctx, pathpkg.Dir(cleaned), []string{pathpkg.Base(cleaned)})
}

func (s *Session) UploadFile(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open openlist upload source %q: %w", localPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat openlist upload source %q: %w", localPath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.baseURL+"/api/fs/put", file)
	if err != nil {
		return fmt.Errorf("build openlist upload request: %w", err)
	}
	req.ContentLength = info.Size()
	req.Header.Set("Authorization", s.token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("File-Path", NormalizePath(remotePath))
	req.Header.Set("Overwrite", "true")
	req.Header.Set("Last-Modified", fmt.Sprintf("%d", info.ModTime().UnixMilli()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openlist upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if _, err := readAndValidateResponse(resp, nil); err != nil {
		return err
	}
	return nil
}

func (s *Session) ResolveDownloadURL(ctx context.Context, remotePath string) (string, error) {
	if linkURL, err := s.resolveLinkAction(ctx, remotePath); err == nil && strings.TrimSpace(linkURL) != "" {
		return linkURL, nil
	}

	object, err := s.Get(ctx, remotePath)
	if err != nil {
		return "", err
	}
	if object.IsDir {
		return "", fmt.Errorf("openlist path %q is a directory", NormalizePath(remotePath))
	}
	return s.directDownloadURL(remotePath, object.Sign), nil
}

func (s *Session) OpenDownload(ctx context.Context, remotePath, rangeHeader string) (*http.Response, error) {
	downloadURL, err := s.ResolveDownloadURL(ctx, remotePath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build openlist download request: %w", err)
	}
	if strings.TrimSpace(rangeHeader) != "" {
		req.Header.Set("Range", strings.TrimSpace(rangeHeader))
	}
	if sameOrigin(s.baseURL, downloadURL) {
		req.Header.Set("Authorization", s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openlist download request failed: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		return nil, fmt.Errorf("openlist download failed: %s", responseMessage(resp.Status, strings.TrimSpace(string(body))))
	}
	return resp, nil
}

func (s *Session) postJSON(ctx context.Context, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openlist request %s: %w", endpoint, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build openlist request %s: %w", endpoint, err)
	}
	req.Header.Set("Authorization", s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openlist request %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	data, err := readAndValidateResponse(resp, out)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode openlist response %s: %w", endpoint, err)
	}
	return nil
}

func (s *Session) resolveLinkAction(ctx context.Context, remotePath string) (string, error) {
	linkURL := s.baseURL + "/@file/link/path/" + strings.TrimPrefix(EncodePathForRoute(remotePath), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, linkURL, nil)
	if err != nil {
		return "", fmt.Errorf("build openlist link request: %w", err)
	}
	req.Header.Set("Authorization", s.token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openlist link request failed: %w", err)
	}
	defer resp.Body.Close()

	if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" && resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode < http.StatusBadRequest {
		return resolveRelativeURL(s.baseURL, location), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", fmt.Errorf("read openlist link response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if isNotFoundMessage(strings.TrimSpace(string(body))) {
		return "", ErrNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("openlist link request failed: %s", responseMessage(resp.Status, strings.TrimSpace(string(body))))
	}

	resolved, ok := extractLinkURL(body)
	if !ok {
		return "", fmt.Errorf("openlist link response did not contain a download url")
	}
	return resolveRelativeURL(s.baseURL, resolved), nil
}

func (s *Session) directDownloadURL(remotePath, sign string) string {
	base := s.baseURL + "/d" + EncodePathForRoute(remotePath)
	if strings.TrimSpace(sign) == "" {
		return base
	}
	return base + "?sign=" + url.QueryEscape(strings.TrimSpace(sign))
}

func readAndValidateResponse(resp *http.Response, out any) (json.RawMessage, error) {
	var envelope apiEnvelope
	if err := decodeEnvelope(resp, &envelope); err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound || envelope.Code == http.StatusNotFound || isNotFoundMessage(envelope.Message) {
		return nil, ErrNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || envelope.Code != http.StatusOK {
		return nil, fmt.Errorf("openlist request failed: %s", responseMessage(resp.Status, envelope.Message))
	}
	if out == nil {
		return nil, nil
	}
	return envelope.Data, nil
}

func decodeEnvelope(resp *http.Response, envelope *apiEnvelope) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return fmt.Errorf("read openlist response: %w", err)
	}
	if err := json.Unmarshal(body, envelope); err != nil {
		return fmt.Errorf("decode openlist response: %w", err)
	}
	return nil
}

func extractLinkURL(body []byte) (string, bool) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", false
	}
	if looksLikeURL(trimmed) {
		return trimmed, true
	}

	var rawString string
	if err := json.Unmarshal(body, &rawString); err == nil && looksLikeURL(strings.TrimSpace(rawString)) {
		return strings.TrimSpace(rawString), true
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	return findURL(payload)
}

func findURL(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, looksLikeURL(trimmed)
	case map[string]any:
		for _, key := range []string{"data", "url", "href", "link", "raw_url"} {
			if candidate, ok := typed[key]; ok {
				if resolved, found := findURL(candidate); found {
					return resolved, true
				}
			}
		}
	}
	return "", false
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/")
}

func NormalizePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "/"
	}
	cleaned := pathpkg.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func EncodePathForRoute(raw string) string {
	cleaned := NormalizePath(raw)
	if cleaned == "/" {
		return "/"
	}
	segments := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return "/" + strings.Join(segments, "/")
}

func responseMessage(statusText, message string) string {
	message = strings.TrimSpace(message)
	if message != "" {
		return message
	}
	return strings.TrimSpace(statusText)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isAlreadyExistsError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "already exists") || strings.Contains(message, "exists")
}

func isNotFoundMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(normalized, "not found") || strings.Contains(normalized, "object not found")
}

func resolveRelativeURL(baseURL, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.IsAbs() {
		return trimmed
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return trimmed
	}
	return base.ResolveReference(parsed).String()
}

func sameOrigin(baseURL, requestURL string) bool {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(requestURL))
	if err != nil {
		return false
	}
	if !parsed.IsAbs() {
		return true
	}
	return strings.EqualFold(base.Scheme, parsed.Scheme) && strings.EqualFold(base.Host, parsed.Host)
}
