package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
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
	smtpHostKey       = "smtp.host"
	smtpPortKey       = "smtp.port"
	smtpUsernameKey   = "smtp.username"
	smtpPasswordKey   = "smtp.password"
	smtpFromKey       = "smtp.from"
	smtpEncryptionKey = "smtp.encryption"
)

type SendMailFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

var DefaultSendMail SendMailFunc = smtp.SendMail

const smtpDialTimeout = 15 * time.Second

func dialSMTP(addr, host, encryption string) (net.Conn, error) {
	switch encryption {
	case "ssltls":
		return tls.DialWithDialer(&net.Dialer{Timeout: smtpDialTimeout}, "tcp", addr, &tls.Config{ServerName: host})
	default:
		return net.DialTimeout("tcp", addr, smtpDialTimeout)
	}
}

func sendMailDirect(addr string, auth smtp.Auth, from string, to []string, msg []byte, encryption string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	conn, err := dialSMTP(addr, host, encryption)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer c.Close()
	if encryption == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

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

	return SendSMTPMail(config.Host, config.Port, config.Username, config.Password, config.From, config.Encryption, job.To, job.Subject, job.Body, s.sendMail)
}

func (s *EmailSender) loadSMTPConfig() (*smtpRuntimeConfig, error) {
	values, err := s.db.GetSystemConfigs([]string{smtpHostKey, smtpPortKey, smtpUsernameKey, smtpPasswordKey, smtpFromKey, smtpEncryptionKey})
	if err != nil {
		return nil, err
	}

	encryption := strings.TrimSpace(values[smtpEncryptionKey])
	if encryption == "" {
		encryption = "none"
	}
	config := &smtpRuntimeConfig{
		Host:       strings.TrimSpace(values[smtpHostKey]),
		Username:   strings.TrimSpace(values[smtpUsernameKey]),
		From:       strings.TrimSpace(values[smtpFromKey]),
		Encryption: encryption,
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
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	Encryption string
}

func (c smtpRuntimeConfig) isConfigured() bool {
	return c.Host != "" && c.Port > 0 && c.From != ""
}

func SendSMTPMail(host string, port int, username, password, from, encryption, to, subject, body string, sendMail SendMailFunc) error {
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

	if encryption == "starttls" || encryption == "ssltls" {
		return sendMailDirect(address, auth, trimmedFrom, []string{trimmedTo}, []byte(message), encryption)
	}

	if err := sendMail(address, auth, trimmedFrom, []string{trimmedTo}, []byte(message)); err != nil {
		return fmt.Errorf("send smtp mail: %w", err)
	}

	return nil
}

func buildMailMessage(from, to, subject, body string) string {
	boundary := fmt.Sprintf("rbs-alt-%d", time.Now().UnixNano())
	htmlBody := buildHTMLMailBody(subject, body)
	return strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary),
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		htmlBody,
		"",
		fmt.Sprintf("--%s--", boundary),
	}, "\r\n")
}

type mailDetail struct {
	Label string
	Value string
}

func buildHTMLMailBody(subject, body string) string {
	details, paragraphs := splitMailBody(body)
	accent := "#2fc7f0"
	badge := "系统通知"
	summary := "以下是本次邮件的关键信息。"

	subjectLower := strings.ToLower(strings.TrimSpace(subject))
	switch {
	case strings.Contains(subjectLower, "登录密码") || findMailDetailValue(details, "您的初始登录密码为") != "":
		accent = "#5dcc96"
		badge = "账号信息"
		summary = "系统已为您生成初始登录密码。为确保账户安全，请在首次登录后尽快修改密码。"
	case strings.Contains(subjectLower, "smtp 测试邮件"):
		accent = "#5dcc96"
		badge = "SMTP 测试"
		summary = "这是一封用于确认邮件通知通道是否正常工作的测试邮件。"
	default:
		if severity := findMailDetailValue(details, "风险等级"); severity != "" {
			badge = severity
			accent = riskAccentColor(severity)
		}
		if description := findMailDetailValue(details, "风险描述"); description != "" {
			summary = description
		} else if len(paragraphs) > 0 {
			summary = paragraphs[0]
		}
	}

	content := renderMailDetailTable(details, accent)
	if content == "" {
		content = renderMailParagraphs(paragraphs)
	}
	if content == "" {
		content = `<p style="margin:0;font-size:14px;line-height:1.7;color:#475569;">暂无正文内容。</p>`
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body style="margin:0;padding:24px;background:#f4f8fc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#0f172a;">
    <div style="max-width:680px;margin:0 auto;background:#ffffff;border:1px solid #dbe5f0;border-radius:20px;overflow:hidden;box-shadow:0 18px 40px rgba(15,23,42,0.08);">
      <div style="padding:28px 32px;background:linear-gradient(135deg,%s 0%%, #eaf8ff 100%%);border-bottom:1px solid rgba(15,23,42,0.08);">
        <div style="display:inline-flex;align-items:center;padding:6px 12px;border-radius:999px;background:rgba(255,255,255,0.72);font-size:12px;font-weight:700;letter-spacing:0.04em;color:#0f172a;">%s</div>
        <h1 style="margin:16px 0 10px;font-size:24px;line-height:1.35;color:#0f172a;">%s</h1>
        <p style="margin:0;font-size:14px;line-height:1.8;color:#334155;">%s</p>
      </div>
      <div style="padding:28px 32px;">%s</div>
      <div style="padding:18px 32px;border-top:1px solid #e2e8f0;background:#f8fafc;">
        <p style="margin:0;font-size:12px;line-height:1.7;color:#64748b;">此邮件由 Rsync Backup Service 自动发送，请勿直接回复。</p>
      </div>
    </div>
  </body>
</html>`,
		html.EscapeString(subject),
		accent,
		html.EscapeString(badge),
		html.EscapeString(subject),
		html.EscapeString(summary),
		content,
	)
}

func splitMailBody(body string) ([]mailDetail, []string) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	details := make([]mailDetail, 0, len(lines))
	paragraphs := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		separator := ":"
		idx := strings.Index(line, separator)
		if idx < 0 {
			separator = "："
			idx = strings.Index(line, separator)
		}
		if idx > 0 {
			label := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+len(separator):])
			if label != "" && value != "" {
				details = append(details, mailDetail{Label: label, Value: value})
				continue
			}
		}
		paragraphs = append(paragraphs, line)
	}
	return details, paragraphs
}

func renderMailDetailTable(details []mailDetail, accent string) string {
	if len(details) == 0 {
		return ""
	}
	rows := make([]string, 0, len(details))
	for _, detail := range details {
		rows = append(rows, fmt.Sprintf(
			`<tr><td style="padding:12px 0;border-bottom:1px solid #e2e8f0;width:112px;vertical-align:top;font-size:12px;font-weight:700;color:#64748b;letter-spacing:0.03em;">%s</td><td style="padding:12px 0;border-bottom:1px solid #e2e8f0;font-size:14px;line-height:1.75;color:#0f172a;">%s</td></tr>`,
			html.EscapeString(detail.Label),
			renderMailDetailValue(detail, accent),
		))
	}
	return `<table role="presentation" style="width:100%%;border-collapse:collapse;">` + strings.Join(rows, "") + `</table>`
}

func renderMailDetailValue(detail mailDetail, accent string) string {
	value := html.EscapeString(detail.Value)
	if strings.Contains(detail.Label, "密码") {
		return `<span style="display:inline-flex;align-items:center;padding:8px 12px;border-radius:12px;background:#0f172a;color:#f8fafc;font-size:15px;font-weight:700;letter-spacing:0.06em;font-family:'SFMono-Regular','Cascadia Code','Roboto Mono',monospace;">` + value + `</span>`
	}
	if detail.Label == "风险等级" {
		return `<span style="display:inline-flex;align-items:center;padding:6px 10px;border-radius:999px;background:` + html.EscapeString(accent) + `1f;color:` + html.EscapeString(accent) + `;font-size:12px;font-weight:700;">` + value + `</span>`
	}
	return value
}

func renderMailParagraphs(paragraphs []string) string {
	if len(paragraphs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		parts = append(parts, `<p style="margin:0 0 12px;font-size:14px;line-height:1.8;color:#334155;">`+html.EscapeString(paragraph)+`</p>`)
	}
	return strings.Join(parts, "")
}

func findMailDetailValue(details []mailDetail, label string) string {
	for _, detail := range details {
		if detail.Label == label {
			return detail.Value
		}
	}
	return ""
}

func riskAccentColor(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "严重", "critical":
		return "#f06060"
	case "警告", "warning":
		return "#f5be58"
	case "提示", "info":
		return "#2fc7f0"
	default:
		return "#2fc7f0"
	}
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