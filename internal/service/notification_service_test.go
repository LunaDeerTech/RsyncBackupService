package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/notify"
)

func TestNotificationServiceNotifyFiltersSubscribersByPermissionAndEvent(t *testing.T) {
	fixture := newAuthServiceTestFixture(t)
	viewer := createAuthServiceTestUser(t, fixture.db, "viewer", "viewer-secret", false)
	stale := createAuthServiceTestUser(t, fixture.db, "stale", "stale-secret", false)
	successOnly := createAuthServiceTestUser(t, fixture.db, "success-only", "success-secret", false)

	instance := model.BackupInstance{
		Name:            "db-prod",
		SourceType:      SourceTypeLocal,
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       fixture.admin.ID,
	}
	if err := fixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	permissions := []model.InstancePermission{
		{UserID: viewer.ID, InstanceID: instance.ID, Role: RoleViewer},
		{UserID: successOnly.ID, InstanceID: instance.ID, Role: RoleViewer},
	}
	if err := fixture.db.Create(&permissions).Error; err != nil {
		t.Fatalf("create permissions: %v", err)
	}

	channel := model.NotificationChannel{
		Name:    "smtp-main",
		Type:    "smtp",
		Config:  `{"host":"smtp.example.com","port":587,"username":"mailer","password":"secret","from":"backup@example.com","tls":true}`,
		Enabled: true,
	}
	if err := fixture.db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}

	subscriptions := []model.NotificationSubscription{
		{
			UserID:        viewer.ID,
			InstanceID:    instance.ID,
			ChannelID:     channel.ID,
			Events:        `["backup_failed"]`,
			ChannelConfig: `{"email":"viewer@example.com"}`,
			Enabled:       true,
		},
		{
			UserID:        stale.ID,
			InstanceID:    instance.ID,
			ChannelID:     channel.ID,
			Events:        `["backup_failed"]`,
			ChannelConfig: `{"email":"stale@example.com"}`,
			Enabled:       true,
		},
		{
			UserID:        successOnly.ID,
			InstanceID:    instance.ID,
			ChannelID:     channel.ID,
			Events:        `["backup_success"]`,
			ChannelConfig: `{"email":"success@example.com"}`,
			Enabled:       true,
		},
	}
	if err := fixture.db.Create(&subscriptions).Error; err != nil {
		t.Fatalf("create subscriptions: %v", err)
	}

	svc := NewNotificationService(fixture.db)
	var recipients []string
	svc.buildNotifier = func(channel model.NotificationChannel, subscription model.NotificationSubscription) (notify.Notifier, error) {
		var recipient struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal([]byte(subscription.ChannelConfig), &recipient); err != nil {
			t.Fatalf("decode subscription channel config: %v", err)
		}
		return &stubServiceNotifier{recipient: recipient.Email, recipients: &recipients}, nil
	}

	err := svc.Notify(context.Background(), notify.NotifyEvent{
		Type:       "backup_failed",
		Instance:   instance.Name,
		Strategy:   "nightly",
		Message:    "backup failed",
		Detail:     map[string]any{"instance_id": instance.ID},
		OccurredAt: fixture.now,
	})
	if err != nil {
		t.Fatalf("notify subscribers: %v", err)
	}
	if len(recipients) != 1 || recipients[0] != "viewer@example.com" {
		t.Fatalf("expected only viewer@example.com to receive the notification, got %v", recipients)
	}
}

type stubServiceNotifier struct {
	recipient  string
	recipients *[]string
}

func (n *stubServiceNotifier) Type() string {
	return "smtp"
}

func (n *stubServiceNotifier) Send(ctx context.Context, event notify.NotifyEvent) error {
	_ = ctx
	_ = event
	*n.recipients = append(*n.recipients, n.recipient)
	return nil
}

func (n *stubServiceNotifier) Validate(config json.RawMessage) error {
	_ = config
	return nil
}