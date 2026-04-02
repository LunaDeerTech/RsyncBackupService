package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"mime"
	"strings"
	"time"
)

var emailTemplate = template.Must(template.New("notification-email").Parse(`<!DOCTYPE html>
<html lang="en">
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: #172033; background: #f4f7fb; padding: 24px;">
  <div style="max-width: 640px; margin: 0 auto; background: #ffffff; border-radius: 16px; padding: 24px; border: 1px solid #d9e4ef;">
    <p style="margin: 0 0 8px; font-size: 12px; letter-spacing: 0.08em; text-transform: uppercase; color: #4f7a8f;">Rsync Backup Service</p>
    <h1 style="margin: 0 0 16px; font-size: 24px; line-height: 1.3; color: #172033;">{{.Headline}}</h1>
    <p style="margin: 0 0 16px; font-size: 15px; line-height: 1.6; color: #31465c;">{{.Message}}</p>
    <table style="width: 100%; border-collapse: collapse; font-size: 14px; color: #31465c;">
      <tr>
        <td style="padding: 8px 0; width: 120px; font-weight: 600;">Event</td>
        <td style="padding: 8px 0;">{{.EventType}}</td>
      </tr>
      <tr>
        <td style="padding: 8px 0; font-weight: 600;">Instance</td>
        <td style="padding: 8px 0;">{{.Instance}}</td>
      </tr>
      {{if .Strategy}}
      <tr>
        <td style="padding: 8px 0; font-weight: 600;">Strategy</td>
        <td style="padding: 8px 0;">{{.Strategy}}</td>
      </tr>
      {{end}}
      <tr>
        <td style="padding: 8px 0; font-weight: 600;">Occurred At</td>
        <td style="padding: 8px 0;">{{.OccurredAt}}</td>
      </tr>
    </table>
  </div>
</body>
</html>`))

type emailTemplateData struct {
	Headline   string
	EventType  string
	Instance   string
	Strategy   string
	Message    string
	OccurredAt string
}

func renderEmail(event NotifyEvent) (string, string, error) {
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	data := emailTemplateData{
		Headline:   subjectForEvent(event),
		EventType:  strings.TrimSpace(event.Type),
		Instance:   strings.TrimSpace(event.Instance),
		Strategy:   strings.TrimSpace(event.Strategy),
		Message:    strings.TrimSpace(event.Message),
		OccurredAt: occurredAt.Format(time.RFC3339),
	}

	var body bytes.Buffer
	if err := emailTemplate.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("render notification email: %w", err)
	}

	return mime.QEncoding.Encode("utf-8", data.Headline), body.String(), nil
}

func subjectForEvent(event NotifyEvent) string {
	instance := strings.TrimSpace(event.Instance)
	if instance == "" {
		instance = "unknown instance"
	}

	switch strings.TrimSpace(event.Type) {
	case "backup_success":
		return fmt.Sprintf("Backup Succeeded: %s", instance)
	case "backup_failed":
		return fmt.Sprintf("Backup Failed: %s", instance)
	case "restore_complete":
		return fmt.Sprintf("Restore Succeeded: %s", instance)
	case "restore_failed":
		return fmt.Sprintf("Restore Failed: %s", instance)
	case "notification_test":
		return fmt.Sprintf("Notification Test: %s", instance)
	default:
		return fmt.Sprintf("Notification: %s", instance)
	}
}