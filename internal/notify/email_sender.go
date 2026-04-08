package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/store"
)

const (
	smtpHostKey     = "smtp.host"
	smtpPortKey     = "smtp.port"
	smtpUsernameKey = "smtp.username"
	smtpPasswordKey = "smtp.password"
	smtpFromKey     = "smtp.from"
)

type SendMailFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

var DefaultSendMail SendMailFunc = smtp.SendMail

type EmailSender struct {
	db          *store.DB
	aesKey      []byte
	mu          sync.Mutex
	queue       chan *EmailJob
	started     bool
	sendMail    SendMailFunc
	retryDelays []time.Duration
	sleep       func(context.Context, time.Duration) error
}

type EmailJob struct {
	To      string
	Subject string
	Body    string
	Retries int
}

func NewEmailSender(db *store.DB, aesKey []byte) *EmailSender {
	keyCopy := make([]byte, len(aesKey))
	copy(keyCopy, aesKey)
	return &EmailSender{
		db:          db,
		aesKey:      keyCopy,
		queue:       make(chan *EmailJob, 128),
		sendMail:    DefaultSendMail,
		retryDelays: []time.Duration{5 * time.Second, 25 * time.Second, 125 * time.Second},
		sleep:       sleepWithContext,
	}
}

func (s *EmailSender) Start(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run(ctx)
}

func (s *EmailSender) Send(to, subject, body string) {
	if s == nil {
		return
	}
	job := &EmailJob{
		To:      strings.TrimSpace(to),
		Subject: strings.TrimSpace(subject),
		Body:    strings.TrimSpace(body),
	}
	if job.To == "" || job.Subject == "" || job.Body == "" {
		slog.Warn("skip email job with empty fields", "to", job.To, "subject", job.Subject)
		return
	}
	s.queue <- job
}

func (s *EmailSender) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			s.processJob(ctx, job)
		}
	}
}

func (s *EmailSender) processJob(ctx context.Context, job *EmailJob) {
	if job == nil {
		return
	}

	for attempt := 0; ; attempt++ {
		job.Retries = attempt
		err := s.deliver(job)
		if err == nil {
			return
		}
		if attempt >= len(s.retryDelays) {
			slog.Error("email delivery failed after retries", "to", job.To, "subject", job.Subject, "retries", attempt, "error", err)
			return
		}
		slog.Warn("email delivery failed; retry scheduled", "to", job.To, "subject", job.Subject, "retry", attempt+1, "error", err)
		if err := s.sleep(ctx, s.retryDelays[attempt]); err != nil {
			return
		}
	}
}

func (s *EmailSender) deliver(job *EmailJob) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("email sender unavailable")
	}
	config, err := s.loadSMTPConfig()
	if err != nil {
		return err
	}
	if !config.isConfigured() {
		slog.Warn("smtp not configured; email notification logged", "to", job.To, "subject", job.Subject, "body", job.Body)
		return nil
	}

	return SendSMTPMail(config.Host, config.Port, config.Username, config.Password, config.From, job.To, job.Subject, job.Body, s.sendMail)
}

func (s *EmailSender) loadSMTPConfig() (*smtpRuntimeConfig, error) {
	values, err := s.db.GetSystemConfigs([]string{smtpHostKey, smtpPortKey, smtpUsernameKey, smtpPasswordKey, smtpFromKey})
	if err != nil {
		return nil, err
	}

	config := &smtpRuntimeConfig{
		Host:     strings.TrimSpace(values[smtpHostKey]),
		Username: strings.TrimSpace(values[smtpUsernameKey]),
		From:     strings.TrimSpace(values[smtpFromKey]),
	}
	if rawPort := strings.TrimSpace(values[smtpPortKey]); rawPort != "" {
		if _, err := fmt.Sscanf(rawPort, "%d", &config.Port); err != nil {
			return nil, fmt.Errorf("parse smtp port %q: %w", rawPort, err)
		}
	}
	if encryptedPassword := strings.TrimSpace(values[smtpPasswordKey]); encryptedPassword != "" {
		password, err := authcrypto.AESDecrypt(encryptedPassword, s.aesKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt smtp password: %w", err)
		}
		config.Password = password
	}

	return config, nil
}

type smtpRuntimeConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (c smtpRuntimeConfig) isConfigured() bool {
	return c.Host != "" && c.Port > 0 && c.From != ""
}

func SendSMTPMail(host string, port int, username, password, from, to, subject, body string, sendMail SendMailFunc) error {
	if sendMail == nil {
		sendMail = DefaultSendMail
	}
	trimmedHost := strings.TrimSpace(host)
	trimmedFrom := strings.TrimSpace(from)
	trimmedTo := strings.TrimSpace(to)
	trimmedSubject := strings.TrimSpace(subject)
	trimmedBody := strings.TrimSpace(body)
	if trimmedHost == "" || port <= 0 || trimmedFrom == "" || trimmedTo == "" || trimmedSubject == "" || trimmedBody == "" {
		return fmt.Errorf("smtp mail parameters are incomplete")
	}
	if (strings.TrimSpace(username) == "") != (password == "") {
		return fmt.Errorf("smtp username and password must both be set")
	}

	var auth smtp.Auth
	if strings.TrimSpace(username) != "" {
		auth = smtp.PlainAuth("", strings.TrimSpace(username), password, trimmedHost)
	}

	message := buildMailMessage(trimmedFrom, trimmedTo, trimmedSubject, trimmedBody)
	address := net.JoinHostPort(trimmedHost, fmt.Sprintf("%d", port))
	if err := sendMail(address, auth, trimmedFrom, []string{trimmedTo}, []byte(message)); err != nil {
		return fmt.Errorf("send smtp mail: %w", err)
	}

	return nil
}

func buildMailMessage(from, to, subject, body string) string {
	return strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}