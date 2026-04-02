package notify

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSMTPNotifierValidatesConfig(t *testing.T) {
	notifier := SMTPNotifier{}

	err := notifier.Validate(json.RawMessage(`{"host":"","port":587,"username":"mailer","password":"secret","from":"backup@example.com","tls":true}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSMTPNotifierSendRetriesWithBackoff(t *testing.T) {
	notifier := SMTPNotifier{
		config: SMTPConfig{
			Host:     "smtp.example.com",
			Port:     587,
			Username: "mailer",
			Password: "secret",
			From:     "backup@example.com",
			TLS:      true,
		},
		recipient: "ops@example.com",
	}

	attempts := 0
	var sleeps []time.Duration
	notifier.sendMail = func(ctx context.Context, config SMTPConfig, recipient string, message []byte) error {
		attempts++
		if recipient != "ops@example.com" {
			t.Fatalf("expected recipient ops@example.com, got %q", recipient)
		}
		if attempts < 3 {
			return errors.New("temporary smtp failure")
		}
		return nil
	}
	notifier.sleep = func(ctx context.Context, duration time.Duration) error {
		sleeps = append(sleeps, duration)
		return nil
	}

	err := notifier.Send(context.Background(), NotifyEvent{
		Type:       "backup_failed",
		Instance:   "db-prod",
		Strategy:   "nightly",
		Message:    "backup failed",
		OccurredAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected send to eventually succeed, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if !reflect.DeepEqual(sleeps, []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("expected exponential backoff sleeps [1s 2s], got %v", sleeps)
	}
}

func TestSMTPNotifierSendFailsAfterMaxRetries(t *testing.T) {
	notifier := SMTPNotifier{
		config: SMTPConfig{
			Host:     "smtp.example.com",
			Port:     587,
			Username: "mailer",
			Password: "secret",
			From:     "backup@example.com",
			TLS:      true,
		},
		recipient: "ops@example.com",
	}

	attempts := 0
	var sleeps []time.Duration
	notifier.sendMail = func(ctx context.Context, config SMTPConfig, recipient string, message []byte) error {
		attempts++
		return errors.New("temporary smtp failure")
	}
	notifier.sleep = func(ctx context.Context, duration time.Duration) error {
		sleeps = append(sleeps, duration)
		return nil
	}

	err := notifier.Send(context.Background(), NotifyEvent{
		Type:       "backup_failed",
		Instance:   "db-prod",
		Strategy:   "nightly",
		Message:    "backup failed",
		OccurredAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected send to fail after retries")
	}
	if attempts != 4 {
		t.Fatalf("expected 4 attempts including retries, got %d", attempts)
	}
	if !reflect.DeepEqual(sleeps, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}) {
		t.Fatalf("expected exponential backoff sleeps [1s 2s 4s], got %v", sleeps)
	}
}

func TestSMTPNotifierSendDoesNotRetryPermanentErrors(t *testing.T) {
	notifier := SMTPNotifier{
		config: SMTPConfig{
			Host:     "smtp.example.com",
			Port:     587,
			Username: "mailer",
			Password: "secret",
			From:     "backup@example.com",
			TLS:      true,
		},
		recipient: "ops@example.com",
	}

	attempts := 0
	var sleeps []time.Duration
	notifier.sendMail = func(ctx context.Context, config SMTPConfig, recipient string, message []byte) error {
		attempts++
		return permanentSMTPError{err: errors.New("smtp server does not support STARTTLS")}
	}
	notifier.sleep = func(ctx context.Context, duration time.Duration) error {
		sleeps = append(sleeps, duration)
		return nil
	}

	err := notifier.Send(context.Background(), NotifyEvent{
		Type:       "backup_failed",
		Instance:   "db-prod",
		Strategy:   "nightly",
		Message:    "backup failed",
		OccurredAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected send to fail on a permanent smtp error")
	}
	if attempts != 1 {
		t.Fatalf("expected a permanent smtp error to stop retries after 1 attempt, got %d", attempts)
	}
	if len(sleeps) != 0 {
		t.Fatalf("expected no backoff sleeps for a permanent smtp error, got %v", sleeps)
	}
}