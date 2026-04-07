package model

import "time"

type Task struct {
	ID           int64      `json:"id"`
	InstanceID   int64      `json:"instance_id"`
	BackupID     *int64     `json:"backup_id,omitempty"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Progress     int        `json:"progress"`
	CurrentStep  string     `json:"current_step"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	EstimatedEnd *time.Time `json:"estimated_end,omitempty"`
	ErrorMessage string     `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
}
