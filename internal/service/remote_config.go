package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

const defaultSSHTimeout = 5 * time.Second

var (
	ErrInvalidPrivateKey       = errors.New("invalid private key")
	ErrPrivateKeyRequired      = errors.New("private key is required")
	ErrPrivateKeyNotSupported  = errors.New("private key is only supported for ssh remotes")
	ErrSSHTestNotSupported     = errors.New("only ssh remotes support connection testing")
	ErrRemoteConfigUnavailable = errors.New("remote config service unavailable")
)

type SSHTester func(context.Context, model.RemoteConfig) error

type RemoteConfigInput struct {
	Name          string
	Type          string
	Host          string
	Port          int
	Username      string
	CloudProvider *string
	CloudConfig   *string
}

type RemoteConfigInUseError struct {
	Usage store.RemoteConfigUsage
}

func (e *RemoteConfigInUseError) Error() string {
	return "remote config is in use"
}

type RemoteConfigService struct {
	db        *store.DB
	keyDir    string
	sshTester SSHTester
	audit     *audit.Logger
}

func NewRemoteConfigService(db *store.DB, dataDir string, sshTester SSHTester) *RemoteConfigService {
	keyDir := filepath.Join(dataDir, "keys")
	if absKeyDir, err := filepath.Abs(keyDir); err == nil {
		keyDir = absKeyDir
	}
	if sshTester == nil {
		sshTester = VerifySSHConnection
	}

	return &RemoteConfigService{
		db:        db,
		keyDir:    keyDir,
		sshTester: sshTester,
	}
}

func (s *RemoteConfigService) SetAuditLogger(logger *audit.Logger) {
	if s == nil {
		return
	}
	s.audit = logger
}

func (s *RemoteConfigService) CreateRemoteConfig(ctx context.Context, input RemoteConfigInput, privateKeyPEM []byte) (*model.RemoteConfig, error) {
	if s == nil || s.db == nil {
		return nil, ErrRemoteConfigUnavailable
	}

	remote := &model.RemoteConfig{
		Name:          input.Name,
		Type:          input.Type,
		Host:          input.Host,
		Port:          input.Port,
		Username:      input.Username,
		CloudProvider: cloneOptionalString(input.CloudProvider),
		CloudConfig:   cloneOptionalString(input.CloudConfig),
	}

	if remote.Type == "ssh" {
		privateKeyPath, err := s.writePrivateKey(privateKeyPEM)
		if err != nil {
			return nil, err
		}
		remote.PrivateKeyPath = privateKeyPath
	}

	if err := s.db.CreateRemoteConfig(remote); err != nil {
		_ = s.deleteManagedPrivateKey(remote.PrivateKeyPath)
		return nil, err
	}
	s.writeAudit(ctx, audit.ActionRemoteCreate, map[string]any{
		"remote_id":      remote.ID,
		"name":           remote.Name,
		"type":           remote.Type,
		"host":           remote.Host,
		"port":           remote.Port,
		"username":       remote.Username,
		"cloud_provider": remote.CloudProvider,
	})

	return remote, nil
}

func (s *RemoteConfigService) UpdateRemoteConfig(ctx context.Context, id int64, input RemoteConfigInput, privateKeyPEM []byte, replacePrivateKey bool) (*model.RemoteConfig, error) {
	if s == nil || s.db == nil {
		return nil, ErrRemoteConfigUnavailable
	}

	current, err := s.db.GetRemoteConfigByID(id)
	if err != nil {
		return nil, err
	}

	updated := &model.RemoteConfig{
		ID:             current.ID,
		Name:           input.Name,
		Type:           input.Type,
		Host:           input.Host,
		Port:           input.Port,
		Username:       input.Username,
		PrivateKeyPath: current.PrivateKeyPath,
		CloudProvider:  cloneOptionalString(input.CloudProvider),
		CloudConfig:    cloneOptionalString(input.CloudConfig),
	}

	newPrivateKeyPath := ""
	deleteCurrentKey := false
	switch updated.Type {
	case "ssh":
		if replacePrivateKey {
			newPrivateKeyPath, err = s.writePrivateKey(privateKeyPEM)
			if err != nil {
				return nil, err
			}
			updated.PrivateKeyPath = newPrivateKeyPath
		}
		if strings.TrimSpace(updated.PrivateKeyPath) == "" {
			return nil, ErrPrivateKeyRequired
		}
	case "cloud":
		if replacePrivateKey {
			return nil, ErrPrivateKeyNotSupported
		}
		updated.PrivateKeyPath = ""
		deleteCurrentKey = current.PrivateKeyPath != ""
	default:
		if replacePrivateKey {
			return nil, ErrPrivateKeyNotSupported
		}
	}

	if err := s.db.UpdateRemoteConfig(updated); err != nil {
		_ = s.deleteManagedPrivateKey(newPrivateKeyPath)
		return nil, err
	}

	if newPrivateKeyPath != "" && current.PrivateKeyPath != newPrivateKeyPath {
		_ = s.deleteManagedPrivateKey(current.PrivateKeyPath)
	}
	if deleteCurrentKey {
		_ = s.deleteManagedPrivateKey(current.PrivateKeyPath)
	}
	s.writeAudit(ctx, audit.ActionRemoteUpdate, map[string]any{
		"remote_id":      updated.ID,
		"name":           updated.Name,
		"type":           updated.Type,
		"host":           updated.Host,
		"port":           updated.Port,
		"username":       updated.Username,
		"cloud_provider": updated.CloudProvider,
	})

	return updated, nil
}

func (s *RemoteConfigService) DeleteRemoteConfig(ctx context.Context, id int64) error {
	if s == nil || s.db == nil {
		return ErrRemoteConfigUnavailable
	}

	remote, err := s.db.GetRemoteConfigByID(id)
	if err != nil {
		return err
	}

	usage, err := s.db.GetRemoteConfigUsage(id)
	if err != nil {
		return err
	}
	if usage.InUse() {
		return &RemoteConfigInUseError{Usage: usage}
	}

	if err := s.db.DeleteRemoteConfig(id); err != nil {
		return err
	}

	if err := s.deleteManagedPrivateKey(remote.PrivateKeyPath); err != nil {
		return err
	}
	s.writeAudit(ctx, audit.ActionRemoteDelete, map[string]any{
		"deleted_remote_id": remote.ID,
		"name":              remote.Name,
		"type":              remote.Type,
		"host":              remote.Host,
		"port":              remote.Port,
		"username":          remote.Username,
	})

	return nil
}

func (s *RemoteConfigService) writeAudit(ctx context.Context, action string, detail any) {
	if s == nil || s.audit == nil {
		return
	}
	userID := int64(0)
	if claims := middleware.GetUser(ctx); claims != nil {
		userID = claims.UserID
	}
	if err := s.audit.LogAction(ctx, 0, userID, action, detail); err != nil {
		slog.Error("write remote config audit log failed", "action", action, "user_id", userID, "error", err)
	}
}

func (s *RemoteConfigService) TestRemoteConfigConnection(ctx context.Context, id int64) error {
	if s == nil || s.db == nil {
		return ErrRemoteConfigUnavailable
	}

	remote, err := s.db.GetRemoteConfigByID(id)
	if err != nil {
		return err
	}
	if remote.Type != "ssh" {
		return ErrSSHTestNotSupported
	}
	if strings.TrimSpace(remote.PrivateKeyPath) == "" {
		return ErrPrivateKeyRequired
	}

	return s.sshTester(ctx, *remote)
}

func VerifySSHConnection(ctx context.Context, remote model.RemoteConfig) error {
	client, err := DialSSHClient(ctx, remote)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session setup failed: %v", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout

	commandDone := make(chan error, 1)
	go func() {
		commandDone <- session.Run("echo ok")
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	case err := <-commandDone:
		if err != nil {
			return fmt.Errorf("ssh command execution failed: %v", err)
		}
	}

	if strings.TrimSpace(stdout.String()) != "ok" {
		return fmt.Errorf("ssh command verification failed")
	}

	return nil
}

func DialSSHClient(ctx context.Context, remote model.RemoteConfig) (*ssh.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if remote.Type != "ssh" {
		return nil, ErrSSHTestNotSupported
	}
	if strings.TrimSpace(remote.Host) == "" || remote.Port < 1 || remote.Port > 65535 || strings.TrimSpace(remote.Username) == "" {
		return nil, fmt.Errorf("ssh remote config is incomplete")
	}
	if strings.TrimSpace(remote.PrivateKeyPath) == "" {
		return nil, ErrPrivateKeyRequired
	}

	signer, err := loadSSHSigner(remote.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            remote.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         defaultSSHTimeout,
	}

	address := net.JoinHostPort(remote.Host, strconv.Itoa(remote.Port))
	dialer := &net.Dialer{Timeout: defaultSSHTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("ssh connection failed: %v", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ssh connection failed: %v", err)
		}
	}

	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, clientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh connection failed: %v", err)
	}
	_ = conn.SetDeadline(time.Time{})

	return ssh.NewClient(clientConn, channels, requests), nil
}

func loadSSHSigner(privateKeyPath string) (ssh.Signer, error) {
	info, err := os.Stat(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("private key file is unavailable")
	}
	if info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("private key file permissions must be 0600")
	}

	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("private key file is unreadable")
	}

	signer, err := ssh.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, ErrInvalidPrivateKey
	}

	return signer, nil
}

func (s *RemoteConfigService) writePrivateKey(privateKeyPEM []byte) (string, error) {
	if err := validatePrivateKeyPEM(privateKeyPEM); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.keyDir, 0o700); err != nil {
		return "", fmt.Errorf("create private key directory failed")
	}

	privateKeyPath := filepath.Join(s.keyDir, uuid.NewString()+".pem")
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0o600); err != nil {
		return "", fmt.Errorf("write private key file failed")
	}
	if err := os.Chmod(privateKeyPath, 0o600); err != nil {
		_ = os.Remove(privateKeyPath)
		return "", fmt.Errorf("set private key permissions failed")
	}

	return privateKeyPath, nil
}

func validatePrivateKeyPEM(privateKeyPEM []byte) error {
	if len(bytes.TrimSpace(privateKeyPEM)) == 0 {
		return ErrPrivateKeyRequired
	}
	if _, err := ssh.ParsePrivateKey(privateKeyPEM); err != nil {
		return ErrInvalidPrivateKey
	}

	return nil
}

func (s *RemoteConfigService) deleteManagedPrivateKey(privateKeyPath string) error {
	if strings.TrimSpace(privateKeyPath) == "" {
		return nil
	}

	managed, err := s.isManagedPrivateKeyPath(privateKeyPath)
	if err != nil {
		return err
	}
	if !managed {
		return nil
	}

	if err := os.Remove(privateKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete private key file failed")
	}

	return nil
}

func (s *RemoteConfigService) isManagedPrivateKeyPath(privateKeyPath string) (bool, error) {
	if strings.TrimSpace(privateKeyPath) == "" {
		return false, nil
	}

	resolvedPath, err := filepath.Abs(privateKeyPath)
	if err != nil {
		return false, fmt.Errorf("resolve private key path failed")
	}
	keyDir := s.keyDir
	if keyDir == "" {
		return false, nil
	}

	relPath, err := filepath.Rel(keyDir, resolvedPath)
	if err != nil {
		return false, fmt.Errorf("resolve managed private key path failed")
	}
	if relPath == "." || strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, string(filepath.Separator)) {
		return false, nil
	}

	return true, nil
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func IsRemoteConfigNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
