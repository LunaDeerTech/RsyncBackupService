package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strings"
)

type PasswordSender interface {
	SendPassword(ctx context.Context, email, password string) error
}

type smtpPasswordSender struct {
	config smtpConfig
}

type smtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewPasswordSender() PasswordSender {
	return &smtpPasswordSender{config: loadSMTPConfigFromEnv()}
}

func (s *smtpPasswordSender) SendPassword(ctx context.Context, email, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !s.config.isConfigured() {
		slog.Info("smtp not configured; generated password logged for manual delivery", "email", email, "password", password)
		return nil
	}

	auth, err := s.config.auth()
	if err != nil {
		slog.Error("smtp configuration invalid; generated password logged for manual delivery", "email", email, "error", err, "password", password)
		return nil
	}

	message := buildMailMessage(s.config.From, email, password)
	address := net.JoinHostPort(s.config.Host, s.config.Port)
	if err := smtp.SendMail(address, auth, s.config.From, []string{email}, []byte(message)); err != nil {
		slog.Error("smtp delivery failed; generated password logged for manual delivery", "email", email, "error", err, "password", password)
		return nil
	}

	return nil
}

func loadSMTPConfigFromEnv() smtpConfig {
	return smtpConfig{
		Host:     strings.TrimSpace(os.Getenv("RBS_SMTP_HOST")),
		Port:     strings.TrimSpace(os.Getenv("RBS_SMTP_PORT")),
		Username: strings.TrimSpace(os.Getenv("RBS_SMTP_USERNAME")),
		Password: os.Getenv("RBS_SMTP_PASSWORD"),
		From:     strings.TrimSpace(os.Getenv("RBS_SMTP_FROM")),
	}
}

func (c smtpConfig) isConfigured() bool {
	return c.Host != "" && c.Port != "" && c.From != ""
}

func (c smtpConfig) auth() (smtp.Auth, error) {
	if c.Username == "" && c.Password == "" {
		return nil, nil
	}
	if c.Username == "" || c.Password == "" {
		return nil, fmt.Errorf("smtp username and password must both be set")
	}

	return smtp.PlainAuth("", c.Username, c.Password, c.Host), nil
}

func buildMailMessage(from, to, password string) string {
	return strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		"Subject: Rsync Backup Service 登录密码",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		fmt.Sprintf("您的初始登录密码为: %s", password),
	}, "\r\n")
}
