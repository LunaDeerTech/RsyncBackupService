package model

import "time"

type DashboardTargetHealthSummary struct {
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Unreachable int `json:"unreachable"`
}

type DashboardOverview struct {
	RunningTasks        int                          `json:"running_tasks"`
	QueuedTasks         int                          `json:"queued_tasks"`
	AbnormalInstances   int                          `json:"abnormal_instances"`
	UnresolvedRisks     int                          `json:"unresolved_risks"`
	SystemDRScore       float64                      `json:"system_dr_score"`
	SystemDRLevel       string                       `json:"system_dr_level"`
	TargetHealthSummary DashboardTargetHealthSummary `json:"target_health_summary"`
	TotalInstances      int                          `json:"total_instances"`
	TotalBackups        int64                        `json:"total_backups"`
}

type DashboardInstanceHealth struct {
	Safe    int `json:"safe"`
	Caution int `json:"caution"`
	Risk    int `json:"risk"`
	Danger  int `json:"danger"`
}

type DailyBackupResult struct {
	Date    string `json:"date"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

type DashboardTrends struct {
	BackupResults  []DailyBackupResult     `json:"backup_results"`
	InstanceHealth DashboardInstanceHealth `json:"instance_health"`
}

type FocusInstance struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	DRScore          float64    `json:"dr_score"`
	DRLevel          string     `json:"dr_level"`
	UnresolvedRisks  int        `json:"unresolved_risks"`
	LastBackupTime   *time.Time `json:"last_backup_time"`
	LastBackupStatus string     `json:"last_backup_status"`
}

type DashboardRiskEvent struct {
	ID           int64      `json:"id"`
	InstanceID   *int64     `json:"instance_id,omitempty"`
	InstanceName string     `json:"instance_name"`
	TargetID     *int64     `json:"target_id,omitempty"`
	TargetName   string     `json:"target_name"`
	Severity     string     `json:"severity"`
	Source       string     `json:"source"`
	Message      string     `json:"message"`
	Resolved     bool       `json:"resolved"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}
