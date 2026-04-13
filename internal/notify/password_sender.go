package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"rsync-backup-service/internal/store"
)

type PasswordSender interface {
	SendPassword(ctx context.Context, email, password string) error
}

type smtpPasswordSender struct {
	db       *store.DB
	aesKey   []byte
	sendMail SendMailFunc
}

func NewPasswordSender(db *store.DB, aesKey []byte) PasswordSender {
	keyCopy := make([]byte, len(aesKey))
	copy(keyCopy, aesKey)
	return &smtpPasswordSender{db: db, aesKey: keyCopy, sendMail: DefaultSendMail}
}

func (s *smtpPasswordSender) SendPassword(ctx context.Context, email, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	config, err := loadSMTPRuntimeConfig(s.db, s.aesKey)
	if err != nil {
		slog.Error("failed to load smtp config; generated password logged for manual delivery", "email", email, "error", err, "password", password)
		return nil
	}

	if !config.isConfigured() {
		slog.Info("smtp not configured; generated password logged for manual delivery", "email", email, "password", password)
		return nil
	}

	auth, err := config.auth()
	if err != nil {
		slog.Error("smtp configuration invalid; generated password logged for manual delivery", "email", email, "error", err, "password", password)
		return nil
	}

	_ = auth
	body := strings.Join([]string{
		fmt.Sprintf("您的初始登录密码为: %s", password),
		"安全建议: 首次登录后请立即修改密码，并妥善保管邮件中的凭证信息。",
	}, "\n")
	if err := SendSMTPMail(config.Host, config.Port, config.Username, config.Password, config.From, config.Encryption, email, "Rsync Backup Service 登录密码", body, s.sendMail); err != nil {
		slog.Error("smtp delivery failed; generated password logged for manual delivery", "email", email, "error", err, "password", password)
		return nil
	}

	return nil
}

func (c smtpRuntimeConfig) auth() (smtp.Auth, error) {
	if c.Username == "" && c.Password == "" {
		return nil, nil
	}
	if c.Username == "" || c.Password == "" {
		return nil, fmt.Errorf("smtp username and password must both be set")
	}

	return smtp.PlainAuth("", c.Username, c.Password, c.Host), nil
}
