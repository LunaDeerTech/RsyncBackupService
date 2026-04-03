package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type CreateSSHKeyRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
}

type TestSSHKeyConnectionRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
}

type SSHKeyService struct {
	db         *gorm.DB
	sshKeyRepo repository.SSHKeyRepository
	dataDir    string
}

const managedSSHKeyDirName = "ssh-keys"

var removeManagedPrivateKeyFile = os.Remove

func NewSSHKeyService(db *gorm.DB, dataDir string) *SSHKeyService {
	return &SSHKeyService{
		db:         db,
		sshKeyRepo: repository.NewSSHKeyRepository(db),
		dataDir:    strings.TrimSpace(dataDir),
	}
}

func (s *SSHKeyService) List(ctx context.Context) ([]model.SSHKey, error) {
	return s.sshKeyRepo.List(ctx)
}

func (s *SSHKeyService) Create(ctx context.Context, req CreateSSHKeyRequest) (model.SSHKey, error) {
	return s.create(ctx, req.Name, req.PrivateKey)
}

func (s *SSHKeyService) Delete(ctx context.Context, id uint) error {
	sshKey, err := s.findSSHKey(ctx, id)
	if err != nil {
		return err
	}

	var storageTargetCount int64
	if err := s.db.WithContext(ctx).Model(&model.StorageTarget{}).Where("ssh_key_id = ?", id).Count(&storageTargetCount).Error; err != nil {
		return fmt.Errorf("count storage targets by ssh key: %w", err)
	}
	if storageTargetCount > 0 {
		return ErrResourceInUse
	}

	var instanceCount int64
	if err := s.db.WithContext(ctx).Model(&model.BackupInstance{}).Where("source_ssh_key_id = ?", id).Count(&instanceCount).Error; err != nil {
		return fmt.Errorf("count backup instances by ssh key: %w", err)
	}
	if instanceCount > 0 {
		return ErrResourceInUse
	}

	stagedDeletePath, err := s.stageManagedPrivateKeyDeletion(sshKey.PrivateKeyPath)
	if err != nil {
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewSSHKeyRepository(tx).Delete(ctx, id); err != nil {
			return err
		}

		if err := s.finalizeManagedPrivateKeyDeletion(stagedDeletePath); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if rollbackErr := s.restoreManagedPrivateKey(stagedDeletePath, sshKey.PrivateKeyPath); rollbackErr != nil {
			return fmt.Errorf("%w: %v", err, rollbackErr)
		}
		return err
	}

	return nil
}

func (s *SSHKeyService) TestConnection(ctx context.Context, id uint, req TestSSHKeyConnectionRequest) error {
	sshKey, err := s.findSSHKey(ctx, id)
	if err != nil {
		return err
	}

	trimmedHost := strings.TrimSpace(req.Host)
	trimmedUser := strings.TrimSpace(req.User)
	if trimmedHost == "" {
		return ErrHostRequired
	}
	if trimmedUser == "" {
		return ErrUserRequired
	}
	if req.Port < 0 {
		return ErrInvalidPort
	}

	backend := storage.NewSSHStorage(storage.SSHConfig{
		Host:           trimmedHost,
		Port:           req.Port,
		User:           trimmedUser,
		PrivateKeyPath: sshKey.PrivateKeyPath,
	})

	if err := backend.TestConnection(ctx); err != nil {
		return normalizeSSHRuntimeError(err)
	}

	return nil
}

func (s *SSHKeyService) create(ctx context.Context, name, privateKey string) (model.SSHKey, error) {
	trimmedName := strings.TrimSpace(name)
	trimmedPrivateKey := strings.TrimSpace(privateKey)
	if trimmedName == "" {
		return model.SSHKey{}, ErrNameRequired
	}
	if trimmedPrivateKey == "" {
		return model.SSHKey{}, ErrPrivateKeyRequired
	}

	return s.createFromPrivateKeyContent(ctx, trimmedName, trimmedPrivateKey)
}

func (s *SSHKeyService) createFromPrivateKeyContent(ctx context.Context, name, privateKey string) (model.SSHKey, error) {
	privateKeyBytes := []byte(privateKey)
	if !strings.HasSuffix(privateKey, "\n") {
		privateKeyBytes = append(privateKeyBytes, '\n')
	}

	privateKeyPath, cleanupFile, err := s.writeManagedPrivateKey(privateKeyBytes)
	if err != nil {
		return model.SSHKey{}, err
	}

	signer, err := storage.LoadSSHSigner(privateKeyPath)
	if err != nil {
		cleanupFile()
		return model.SSHKey{}, normalizeSSHRuntimeError(err)
	}

	sshKey, err := s.persistSSHKey(ctx, name, privateKeyPath, signer)
	if err != nil {
		cleanupFile()
		return model.SSHKey{}, err
	}

	return sshKey, nil
}

func (s *SSHKeyService) persistSSHKey(ctx context.Context, name, privateKeyPath string, signer ssh.Signer) (model.SSHKey, error) {
	sshKey := model.SSHKey{
		Name:           name,
		PrivateKeyPath: privateKeyPath,
		Fingerprint:    ssh.FingerprintSHA256(signer.PublicKey()),
	}
	if err := s.sshKeyRepo.Create(ctx, &sshKey); err != nil {
		return model.SSHKey{}, err
	}

	return sshKey, nil
}

func (s *SSHKeyService) writeManagedPrivateKey(privateKeyBytes []byte) (string, func(), error) {
	managedDir, err := s.managedSSHKeyDir()
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(managedDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("prepare ssh key storage: %w", err)
	}

	file, err := os.CreateTemp(managedDir, "upload-*.pem")
	if err != nil {
		return "", nil, fmt.Errorf("create managed ssh key file: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(file.Name())
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("chmod managed ssh key file: %w", err)
	}
	if _, err := file.Write(privateKeyBytes); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write managed ssh key file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close managed ssh key file: %w", err)
	}

	return file.Name(), cleanup, nil
}


func (s *SSHKeyService) managedSSHKeyDir() (string, error) {
	baseDir := strings.TrimSpace(s.dataDir)
	if baseDir == "" {
		return "", ErrManagedSSHKeyStorageNotConfigured
	}

	return filepath.Join(baseDir, managedSSHKeyDirName), nil
}

func (s *SSHKeyService) stageManagedPrivateKeyDeletion(privateKeyPath string) (string, error) {
	if !s.isManagedPrivateKeyPath(privateKeyPath) {
		return "", nil
	}

	if _, err := os.Stat(privateKeyPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat managed ssh key file: %w", err)
	}

	stagedDeletePath := fmt.Sprintf("%s.pending-delete-%d", privateKeyPath, time.Now().UnixNano())
	if err := os.Rename(privateKeyPath, stagedDeletePath); err != nil {
		return "", fmt.Errorf("stage managed ssh key file deletion: %w", err)
	}

	return stagedDeletePath, nil
}

func (s *SSHKeyService) restoreManagedPrivateKey(stagedDeletePath, privateKeyPath string) error {
	if stagedDeletePath == "" {
		return nil
	}

	if err := os.Rename(stagedDeletePath, privateKeyPath); err != nil {
		return fmt.Errorf("restore managed ssh key file: %w", err)
	}

	return nil
}

func (s *SSHKeyService) finalizeManagedPrivateKeyDeletion(stagedDeletePath string) error {
	if stagedDeletePath == "" {
		return nil
	}

	if err := removeManagedPrivateKeyFile(stagedDeletePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove managed ssh key file: %w", err)
	}

	return nil
}

func (s *SSHKeyService) isManagedPrivateKeyPath(privateKeyPath string) bool {
	trimmedPath := strings.TrimSpace(privateKeyPath)
	if trimmedPath == "" {
		return false
	}
	managedDir, err := s.managedSSHKeyDir()
	if err != nil {
		return false
	}

	relativePath, err := filepath.Rel(filepath.Clean(managedDir), filepath.Clean(trimmedPath))
	if err != nil {
		return false
	}

	return relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(os.PathSeparator))
}

func (s *SSHKeyService) findSSHKey(ctx context.Context, id uint) (model.SSHKey, error) {
	sshKey, err := s.sshKeyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SSHKey{}, ErrSSHKeyNotFound
		}
		return model.SSHKey{}, err
	}

	return sshKey, nil
}