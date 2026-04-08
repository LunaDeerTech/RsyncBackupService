package service

import (
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"database/sql"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/notify"
	"rsync-backup-service/internal/store"
)

const maskedSMTPPassword = "***"

const (
	smtpHostKey         = "smtp.host"
	smtpPortKey         = "smtp.port"
	smtpUsernameKey     = "smtp.username"
	smtpPasswordKey     = "smtp.password"
	smtpFromKey         = "smtp.from"
	registrationKey     = "registration.enabled"
	registrationDefault = true
)

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

type SystemConfigService struct {
	db     *store.DB
	aesKey []byte
	mailer notify.SendMailFunc
}

func NewSystemConfigService(db *store.DB, aesKey []byte) *SystemConfigService {
	keyCopy := make([]byte, len(aesKey))
	copy(keyCopy, aesKey)
	return &SystemConfigService{db: db, aesKey: keyCopy, mailer: notify.DefaultSendMail}
}

func (s *SystemConfigService) SetMailer(mailer notify.SendMailFunc) {
	if s == nil || mailer == nil {
		return
	}
	s.mailer = mailer
}

func (s *SystemConfigService) GetSMTPConfig() (*SMTPConfig, error) {
	return s.loadSMTPConfig(false)
}

func (s *SystemConfigService) GetSMTPConfigWithPassword() (*SMTPConfig, error) {
	return s.loadSMTPConfig(true)
}

func (s *SystemConfigService) UpdateSMTPConfig(cfg *SMTPConfig) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("system config service unavailable")
	}
	validated, err := s.normalizeSMTPConfig(cfg)
	if err != nil {
		return err
	}

	currentPassword := ""
	if validated.Password == maskedSMTPPassword {
		current, err := s.loadSMTPConfig(true)
		if err != nil {
			return err
		}
		currentPassword = current.Password
	}
	if validated.Password == maskedSMTPPassword {
		validated.Password = currentPassword
	}

	encryptedPassword := ""
	if validated.Password != "" {
		encryptedPassword, err = authcrypto.AESEncrypt(validated.Password, s.aesKey)
		if err != nil {
			return fmt.Errorf("encrypt smtp password: %w", err)
		}
	}

	return s.db.SetSystemConfigs(map[string]string{
		smtpHostKey:     validated.Host,
		smtpPortKey:     fmt.Sprintf("%d", validated.Port),
		smtpUsernameKey: validated.Username,
		smtpPasswordKey: encryptedPassword,
		smtpFromKey:     validated.From,
	})
}

func (s *SystemConfigService) TestSMTP(cfg *SMTPConfig, to string) error {
	validated, err := s.normalizeSMTPConfig(cfg)
	if err != nil {
		return err
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("test recipient is required")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("invalid test recipient: %w", err)
	}

	return notify.SendSMTPMail(validated.Host, validated.Port, validated.Username, validated.Password, validated.From, to, "Rsync Backup Service SMTP 测试邮件", "这是一封来自 Rsync Backup Service 的 SMTP 测试邮件。", s.mailer)
}

func (s *SystemConfigService) GetRegistrationEnabled() (bool, error) {
	if s == nil || s.db == nil {
		return registrationDefault, fmt.Errorf("system config service unavailable")
	}
	value, err := s.db.GetSystemConfig(registrationKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return registrationDefault, nil
		}
		return registrationDefault, err
	}

	return strings.EqualFold(strings.TrimSpace(value), "true"), nil
}

func (s *SystemConfigService) UpdateRegistrationEnabled(enabled bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("system config service unavailable")
	}
	return s.db.SetSystemConfig(registrationKey, fmt.Sprintf("%t", enabled))
}

func (s *SystemConfigService) loadSMTPConfig(includePassword bool) (*SMTPConfig, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("system config service unavailable")
	}
	values, err := s.db.GetSystemConfigs([]string{smtpHostKey, smtpPortKey, smtpUsernameKey, smtpPasswordKey, smtpFromKey})
	if err != nil {
		return nil, err
	}

	config := &SMTPConfig{
		Host:     strings.TrimSpace(values[smtpHostKey]),
		Username: strings.TrimSpace(values[smtpUsernameKey]),
		From:     strings.TrimSpace(values[smtpFromKey]),
	}
	if rawPort := strings.TrimSpace(values[smtpPortKey]); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil {
			return nil, fmt.Errorf("parse smtp port %q: %w", rawPort, err)
		}
		config.Port = port
	}
	if encryptedPassword := strings.TrimSpace(values[smtpPasswordKey]); encryptedPassword != "" {
		if includePassword {
			password, err := authcrypto.AESDecrypt(encryptedPassword, s.aesKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt smtp password: %w", err)
			}
			config.Password = password
		} else {
			config.Password = maskedSMTPPassword
		}
	}

	return config, nil
}

func (s *SystemConfigService) normalizeSMTPConfig(cfg *SMTPConfig) (*SMTPConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("smtp config is required")
	}
	normalized := &SMTPConfig{
		Host:     strings.TrimSpace(cfg.Host),
		Port:     cfg.Port,
		Username: strings.TrimSpace(cfg.Username),
		Password: cfg.Password,
		From:     strings.TrimSpace(cfg.From),
	}
	if normalized.Host == "" {
		return nil, fmt.Errorf("smtp host is required")
	}
	if normalized.Port <= 0 {
		return nil, fmt.Errorf("smtp port must be positive")
	}
	if normalized.From == "" {
		return nil, fmt.Errorf("smtp from is required")
	}
	if _, err := mail.ParseAddress(normalized.From); err != nil {
		return nil, fmt.Errorf("invalid smtp from address: %w", err)
	}
	if (normalized.Username == "") != (normalized.Password == "") && normalized.Password != maskedSMTPPassword {
		return nil, fmt.Errorf("smtp username and password must both be set")
	}

	return normalized, nil
}