package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type CreateSSHKeyRequest struct {
	Name           string `json:"name"`
	PrivateKeyPath string `json:"private_key_path"`
}

type TestSSHKeyConnectionRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
}

type SSHKeyService struct {
	db         *gorm.DB
	sshKeyRepo repository.SSHKeyRepository
}

func NewSSHKeyService(db *gorm.DB) *SSHKeyService {
	return &SSHKeyService{
		db:         db,
		sshKeyRepo: repository.NewSSHKeyRepository(db),
	}
}

func (s *SSHKeyService) List(ctx context.Context) ([]model.SSHKey, error) {
	return s.sshKeyRepo.List(ctx)
}

func (s *SSHKeyService) Create(ctx context.Context, req CreateSSHKeyRequest) (model.SSHKey, error) {
	return s.create(ctx, req.Name, req.PrivateKeyPath)
}

func (s *SSHKeyService) Register(ctx context.Context, name, privateKeyPath string) error {
	_, err := s.create(ctx, name, privateKeyPath)
	return err
}

func (s *SSHKeyService) Delete(ctx context.Context, id uint) error {
	if _, err := s.findSSHKey(ctx, id); err != nil {
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

	return s.sshKeyRepo.Delete(ctx, id)
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

func (s *SSHKeyService) create(ctx context.Context, name, privateKeyPath string) (model.SSHKey, error) {
	trimmedName := strings.TrimSpace(name)
	trimmedPrivateKeyPath := strings.TrimSpace(privateKeyPath)
	if trimmedName == "" {
		return model.SSHKey{}, ErrNameRequired
	}
	if trimmedPrivateKeyPath == "" {
		return model.SSHKey{}, ErrPrivateKeyPathRequired
	}

	privateKeyInfo, err := os.Stat(trimmedPrivateKeyPath)
	if err != nil {
		return model.SSHKey{}, fmt.Errorf("stat ssh private key: %w", err)
	}
	if !privateKeyInfo.Mode().IsRegular() {
		return model.SSHKey{}, ErrInvalidSSHKey
	}
	if privateKeyInfo.Mode().Perm() != 0o600 {
		return model.SSHKey{}, fmt.Errorf("%w: got %04o", ErrInvalidSSHKeyPermissions, privateKeyInfo.Mode().Perm())
	}

	privateKeyBytes, err := os.ReadFile(trimmedPrivateKeyPath)
	if err != nil {
		return model.SSHKey{}, fmt.Errorf("read ssh private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return model.SSHKey{}, fmt.Errorf("%w: %v", ErrInvalidSSHKey, err)
	}

	sshKey := model.SSHKey{
		Name:           trimmedName,
		PrivateKeyPath: trimmedPrivateKeyPath,
		Fingerprint:    ssh.FingerprintSHA256(signer.PublicKey()),
	}
	if err := s.sshKeyRepo.Create(ctx, &sshKey); err != nil {
		return model.SSHKey{}, err
	}

	return sshKey, nil
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