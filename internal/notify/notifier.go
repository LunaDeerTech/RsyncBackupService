package notify

import (
	"context"
	"encoding/json"
	"time"
)

const TypeSMTP = "smtp"

type Notifier interface {
	Type() string
	Send(ctx context.Context, event NotifyEvent) error
	Validate(config json.RawMessage) error
}

type NotifyEvent struct {
	Type       string
	Instance   string
	Strategy   string
	Message    string
	Detail     any
	OccurredAt time.Time
}