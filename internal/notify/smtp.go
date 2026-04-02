package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	sendTimeout    = 30 * time.Second
	maxRetryCount  = 3
	initialBackoff = time.Second
)

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	From     string `json:"from"`
	TLS      bool   `json:"tls"`
}

type SMTPRecipientConfig struct {
	Email string `json:"email"`
}

type RedactedSMTPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	From        string `json:"from"`
	TLS         bool   `json:"tls"`
	HasPassword bool   `json:"has_password"`
}

type smtpSendFunc func(ctx context.Context, config SMTPConfig, recipient string, message []byte) error
type smtpSleepFunc func(ctx context.Context, duration time.Duration) error

type permanentSMTPError struct {
	err error
}

func (e permanentSMTPError) Error() string {
	return e.err.Error()
}

func (e permanentSMTPError) Unwrap() error {
	return e.err
}

type SMTPNotifier struct {
	config    SMTPConfig
	recipient string
	sendMail  smtpSendFunc
	sleep     smtpSleepFunc
}

func NewSMTPNotifier(configRaw, recipientRaw json.RawMessage) (*SMTPNotifier, error) {
	config, err := ParseSMTPConfig(configRaw)
	if err != nil {
		return nil, err
	}
	recipientConfig, err := ParseSMTPRecipientConfig(recipientRaw)
	if err != nil {
		return nil, err
	}

	return &SMTPNotifier{
		config:    config,
		recipient: recipientConfig.Email,
		sendMail:  defaultSMTPSend,
		sleep:     sleepWithContext,
	}, nil
}

func (n *SMTPNotifier) Type() string {
	return TypeSMTP
}

func (n *SMTPNotifier) Validate(config json.RawMessage) error {
	_, err := ParseSMTPConfig(config)
	return err
}

func (n *SMTPNotifier) Send(ctx context.Context, event NotifyEvent) error {
	if err := validateSMTPConfig(n.config); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(n.recipient)); err != nil {
		return fmt.Errorf("invalid smtp recipient email: %w", err)
	}
	if n.sendMail == nil {
		n.sendMail = defaultSMTPSend
	}
	if n.sleep == nil {
		n.sleep = sleepWithContext
	}

	subject, htmlBody, err := renderEmail(event)
	if err != nil {
		return err
	}
	message := buildSMTPMessage(n.config.From, n.recipient, subject, htmlBody)

	timedCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= maxRetryCount; attempt++ {
		if err := timedCtx.Err(); err != nil {
			if lastErr != nil {
				return errors.Join(lastErr, err)
			}
			return err
		}

		lastErr = n.sendMail(timedCtx, n.config, n.recipient, message)
		if lastErr == nil {
			return nil
		}
		if !isRetryableSendError(lastErr) || attempt == maxRetryCount {
			return lastErr
		}

		backoff := initialBackoff << attempt
		if err := n.sleep(timedCtx, backoff); err != nil {
			return errors.Join(lastErr, err)
		}
	}

	return lastErr
}

func ParseSMTPConfig(config json.RawMessage) (SMTPConfig, error) {
	var parsedConfig SMTPConfig
	if len(bytes.TrimSpace(config)) == 0 {
		return SMTPConfig{}, errors.New("smtp config is required")
	}
	if err := json.Unmarshal(config, &parsedConfig); err != nil {
		return SMTPConfig{}, fmt.Errorf("decode smtp config: %w", err)
	}
	if err := validateSMTPConfig(parsedConfig); err != nil {
		return SMTPConfig{}, err
	}

	return parsedConfig, nil
}

func ParseSMTPRecipientConfig(config json.RawMessage) (SMTPRecipientConfig, error) {
	var recipientConfig SMTPRecipientConfig
	if len(bytes.TrimSpace(config)) == 0 {
		return SMTPRecipientConfig{}, errors.New("smtp recipient config is required")
	}
	if err := json.Unmarshal(config, &recipientConfig); err != nil {
		return SMTPRecipientConfig{}, fmt.Errorf("decode smtp recipient config: %w", err)
	}
	recipientConfig.Email = strings.TrimSpace(recipientConfig.Email)
	if recipientConfig.Email == "" {
		return SMTPRecipientConfig{}, errors.New("smtp recipient email is required")
	}
	if _, err := mail.ParseAddress(recipientConfig.Email); err != nil {
		return SMTPRecipientConfig{}, fmt.Errorf("invalid smtp recipient email: %w", err)
	}

	return recipientConfig, nil
}

func RedactSMTPConfig(config json.RawMessage) (json.RawMessage, error) {
	parsedConfig, err := ParseSMTPConfig(config)
	if err != nil {
		return nil, err
	}

	encodedConfig, err := json.Marshal(RedactedSMTPConfig{
		Host:        parsedConfig.Host,
		Port:        parsedConfig.Port,
		Username:    parsedConfig.Username,
		From:        parsedConfig.From,
		TLS:         parsedConfig.TLS,
		HasPassword: strings.TrimSpace(parsedConfig.Password) != "",
	})
	if err != nil {
		return nil, fmt.Errorf("encode redacted smtp config: %w", err)
	}

	return encodedConfig, nil
}

func MergeSMTPConfig(currentRaw, nextRaw json.RawMessage) (SMTPConfig, error) {
	var nextMap map[string]json.RawMessage
	if len(bytes.TrimSpace(nextRaw)) == 0 {
		return SMTPConfig{}, errors.New("smtp config is required")
	}
	if err := json.Unmarshal(nextRaw, &nextMap); err != nil {
		return SMTPConfig{}, fmt.Errorf("decode smtp config for merge: %w", err)
	}

	mergedConfig, err := ParseSMTPConfig(nextRaw)
	if err != nil {
		if len(bytes.TrimSpace(currentRaw)) == 0 {
			return SMTPConfig{}, err
		}
		currentConfig, currentErr := ParseSMTPConfig(currentRaw)
		if currentErr != nil {
			return SMTPConfig{}, err
		}
		mergedConfig = currentConfig
		if err := json.Unmarshal(nextRaw, &mergedConfig); err != nil {
			return SMTPConfig{}, fmt.Errorf("decode smtp config for merge: %w", err)
		}
	} else if _, hasPassword := nextMap["password"]; !hasPassword && len(bytes.TrimSpace(currentRaw)) > 0 {
		currentConfig, currentErr := ParseSMTPConfig(currentRaw)
		if currentErr == nil {
			mergedConfig.Password = currentConfig.Password
		}
	}

	if err := validateSMTPConfig(mergedConfig); err != nil {
		return SMTPConfig{}, err
	}

	return mergedConfig, nil
}

func defaultSMTPSend(ctx context.Context, config SMTPConfig, recipient string, message []byte) error {
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	var (
		client *smtp.Client
		conn   net.Conn
		err    error
	)

	dialer := &net.Dialer{Timeout: sendTimeout}
	if config.TLS && config.Port == 465 {
		conn, err = dialTLSContext(ctx, dialer, address, config.Host)
		if err != nil {
			return fmt.Errorf("dial smtp over tls: %w", err)
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
		client, err = smtp.NewClient(conn, config.Host)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("dial smtp: %w", err)
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
		client, err = smtp.NewClient(conn, config.Host)
	}
	if err != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if config.TLS && config.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return permanentSMTPError{err: errors.New("smtp server does not support STARTTLS")}
		}
		if err := client.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.Host}); err != nil {
			return fmt.Errorf("start tls: %w", err)
		}
	}

	if strings.TrimSpace(config.Username) != "" || strings.TrimSpace(config.Password) != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return permanentSMTPError{err: errors.New("smtp server does not support authentication")}
		}
		auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(config.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message writer: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit smtp client: %w", err)
	}

	return nil
}

func dialTLSContext(ctx context.Context, dialer *net.Dialer, address, serverName string) (net.Conn, error) {
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func buildSMTPMessage(from, recipient, subject, htmlBody string) []byte {
	message := strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", recipient),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		htmlBody,
	}, "\r\n")

	return []byte(message)
}

func validateSMTPConfig(config SMTPConfig) error {
	config.Host = strings.TrimSpace(config.Host)
	config.Username = strings.TrimSpace(config.Username)
	config.Password = strings.TrimSpace(config.Password)
	config.From = strings.TrimSpace(config.From)

	if config.Host == "" {
		return errors.New("smtp host is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return errors.New("smtp port must be between 1 and 65535")
	}
	if config.From == "" {
		return errors.New("smtp from address is required")
	}
	if _, err := mail.ParseAddress(config.From); err != nil {
		return fmt.Errorf("invalid smtp from address: %w", err)
	}
	if (config.Username == "") != (config.Password == "") {
		return errors.New("smtp username and password must be provided together")
	}

	return nil
}

func isRetryableSendError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var permanentErr permanentSMTPError
	if errors.As(err, &permanentErr) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var protocolErr *textproto.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code >= 400 && protocolErr.Code < 500
	}

	return true
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}