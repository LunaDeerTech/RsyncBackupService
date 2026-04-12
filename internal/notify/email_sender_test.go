package notify

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/store"
)

func TestEmailSenderRetriesThenSucceeds(t *testing.T) {
	db := newEmailSenderTestDB(t)
	service := map[string]string{
		smtpHostKey: "smtp.example.com",
		smtpPortKey: "587",
		smtpFromKey: "noreply@example.com",
	}
	if err := db.SetSystemConfigs(service); err != nil {
		t.Fatalf("SetSystemConfigs() error = %v", err)
	}

	sender := NewEmailSender(db, authcrypto.DeriveAESKey("secret"))
	sender.retryDelays = []time.Duration{0, 0, 0}
	attempts := 0
	delivered := make(chan struct{}, 1)
	sender.sendMail = func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary smtp failure")
		}
		delivered <- struct{}{}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sender.Start(ctx)
	sender.Send("user@example.com", "Subject", "Body")

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("email sender did not finish delivery")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want %d", attempts, 3)
	}
}

func TestEmailSenderFallsBackWhenSMTPMissing(t *testing.T) {
	db := newEmailSenderTestDB(t)
	sender := NewEmailSender(db, authcrypto.DeriveAESKey("secret"))
	sender.retryDelays = []time.Duration{0, 0, 0}
	called := false
	sender.sendMail = func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		called = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sender.Start(ctx)
	sender.Send("user@example.com", "Subject", "Body")
	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("sendMail was called without smtp config")
	}
}

func TestSendSMTPMailBuildsMultipartHTMLMessage(t *testing.T) {
	called := false
	err := SendSMTPMail(
		"smtp.example.com",
		587,
		"",
		"",
		"noreply@example.com",
		"none",
		"user@example.com",
		"[RBS 预警] 实例 demo 出现备份失败风险",
		"实例: demo\n风险等级: 严重\n风险描述: 最近一次备份执行失败",
		func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
			called = true
			if addr != "smtp.example.com:587" || from != "noreply@example.com" || len(to) != 1 || to[0] != "user@example.com" {
				t.Fatalf("mailer args = %q %q %+v", addr, from, to)
			}
			message := string(msg)
			if !strings.Contains(message, "Content-Type: multipart/alternative") {
				t.Fatalf("message = %q, want multipart alternative content type", message)
			}
			if !strings.Contains(message, "Content-Type: text/html; charset=UTF-8") {
				t.Fatalf("message = %q, want html part", message)
			}
			if !strings.Contains(message, "风险描述") || !strings.Contains(message, "最近一次备份执行失败") {
				t.Fatalf("message = %q, want rendered risk details", message)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("SendSMTPMail() error = %v", err)
	}
	if !called {
		t.Fatal("SendSMTPMail() did not call mailer")
	}
}

func newEmailSenderTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}
	return db
}