package handler

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"testing"
	"time"

	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
	servicepkg "rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestDashboardEndpoints(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin-dashboard@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer-dashboard@example.com", "Viewer", "viewer", "ViewerPass123")
	now := time.Now().UTC().Truncate(time.Second)

	disasterRecovery := servicepkg.NewDisasterRecoveryService(db)
	disasterRecovery.SetClock(func() time.Time { return now })
	scheduler := engine.NewScheduler(db, nil)
	scheduler.SetClock(func() time.Time { return now })
	router := NewRouter(db, WithJWTSecret("secret"), WithScheduler(scheduler), WithDisasterRecoveryService(disasterRecovery))

	healthyTarget := createDashboardTestTarget(t, db, "dashboard-healthy", "healthy")
	createDashboardTestTarget(t, db, "dashboard-degraded", "degraded")
	unreachableTarget := createDashboardTestTarget(t, db, "dashboard-unreachable", "unreachable")

	alpha := createDashboardTestInstance(t, db, "alpha")
	beta := createDashboardTestInstance(t, db, "beta")
	gamma := createDashboardTestInstance(t, db, "gamma")

	alphaPolicy := createDashboardTestPolicy(t, db, alpha.ID, healthyTarget.ID, "alpha-policy", "3600")
	betaPolicy := createDashboardTestPolicy(t, db, beta.ID, healthyTarget.ID, "beta-policy", "7200")
	gammaPolicy := createDashboardTestPolicy(t, db, gamma.ID, healthyTarget.ID, "gamma-policy", "10800")

	createDashboardTestBackup(t, db, alpha.ID, alphaPolicy.ID, "failed", now.Add(-2*time.Hour))
	createDashboardTestBackup(t, db, alpha.ID, alphaPolicy.ID, "failed", now.Add(-26*time.Hour))
	createDashboardTestBackup(t, db, beta.ID, betaPolicy.ID, "success", now.Add(-30*time.Minute))
	createDashboardTestBackup(t, db, gamma.ID, gammaPolicy.ID, "success", now.Add(-15*time.Minute))
	createDashboardTestBackup(t, db, gamma.ID, gammaPolicy.ID, "success", now.Add(-25*time.Hour))

	createDashboardTestTask(t, db, alpha.ID, "rolling", "running")
	createDashboardTestTask(t, db, beta.ID, "rolling", "queued")

	createDashboardTestRiskEvent(t, db, &gamma.ID, &unreachableTarget.ID, model.RiskSeverityCritical, model.RiskSourceTargetUnreachable, "target unreachable", false, now.Add(-10*time.Minute))
	createDashboardTestRiskEvent(t, db, &alpha.ID, &healthyTarget.ID, model.RiskSeverityWarning, model.RiskSourceBackupFailed, "backup failed twice", false, now.Add(-20*time.Minute))
	createDashboardTestRiskEvent(t, db, &alpha.ID, &healthyTarget.ID, model.RiskSeverityInfo, model.RiskSourceColdBackupMissing, "cold backup missing", false, now.Add(-30*time.Minute))
	createDashboardTestRiskEvent(t, db, &beta.ID, nil, model.RiskSeverityInfo, model.RiskSourceBackupFailed, "resolved risk", true, now.Add(-40*time.Minute))

	if err := db.SetInstancePermissions(alpha.ID, []model.InstancePermission{{UserID: viewer.ID, Permission: "readonly"}}); err != nil {
		t.Fatalf("SetInstancePermissions() error = %v", err)
	}

	token := mustAccessTokenForUser(t, admin, "secret")

	overviewResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/overview", nil, token)
	if overviewResponse.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/overview status = %d, want %d, body = %s", overviewResponse.Code, http.StatusOK, overviewResponse.Body.String())
	}
	var overviewEnvelope apiEnvelope
	if err := json.Unmarshal(overviewResponse.Body.Bytes(), &overviewEnvelope); err != nil {
		t.Fatalf("decode overview envelope: %v", err)
	}
	var overview model.DashboardOverview
	if err := json.Unmarshal(overviewEnvelope.Data, &overview); err != nil {
		t.Fatalf("decode overview payload: %v", err)
	}

	alphaScore := mustDashboardScore(t, disasterRecovery, alpha.ID)
	betaScore := mustDashboardScore(t, disasterRecovery, beta.ID)
	gammaScore := mustDashboardScore(t, disasterRecovery, gamma.ID)
	wantAverage := (alphaScore.Total + betaScore.Total + gammaScore.Total) / 3
	wantAbnormal := countAbnormalDashboardScores(alphaScore.Total, betaScore.Total, gammaScore.Total)

	if overview.RunningTasks != 1 {
		t.Fatalf("overview.RunningTasks = %d, want 1", overview.RunningTasks)
	}
	if overview.QueuedTasks != 1 {
		t.Fatalf("overview.QueuedTasks = %d, want 1", overview.QueuedTasks)
	}
	if overview.UnresolvedRisks != 3 {
		t.Fatalf("overview.UnresolvedRisks = %d, want 3", overview.UnresolvedRisks)
	}
	if overview.AbnormalInstances != wantAbnormal {
		t.Fatalf("overview.AbnormalInstances = %d, want %d", overview.AbnormalInstances, wantAbnormal)
	}
	if math.Abs(overview.SystemDRScore-wantAverage) > 0.01 {
		t.Fatalf("overview.SystemDRScore = %.4f, want %.4f", overview.SystemDRScore, wantAverage)
	}
	if overview.SystemDRLevel != servicepkg.DRLevelForScore(wantAverage) {
		t.Fatalf("overview.SystemDRLevel = %q, want %q", overview.SystemDRLevel, servicepkg.DRLevelForScore(wantAverage))
	}
	if overview.TargetHealthSummary.Healthy != 1 || overview.TargetHealthSummary.Degraded != 1 || overview.TargetHealthSummary.Unreachable != 1 {
		t.Fatalf("overview.TargetHealthSummary = %+v, want healthy=1 degraded=1 unreachable=1", overview.TargetHealthSummary)
	}
	if overview.TotalInstances != 3 {
		t.Fatalf("overview.TotalInstances = %d, want 3", overview.TotalInstances)
	}
	if overview.TotalBackups != 5 {
		t.Fatalf("overview.TotalBackups = %d, want 5", overview.TotalBackups)
	}

	viewerOverview := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/overview", nil, mustAccessTokenForUser(t, viewer, "secret"))
	assertAPIError(t, viewerOverview, http.StatusForbidden, 40301, "forbidden")

	risksResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/risks?page=1&page_size=2", nil, token)
	if risksResponse.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/risks status = %d, want %d, body = %s", risksResponse.Code, http.StatusOK, risksResponse.Body.String())
	}
	var risksEnvelope apiEnvelope
	if err := json.Unmarshal(risksResponse.Body.Bytes(), &risksEnvelope); err != nil {
		t.Fatalf("decode risks envelope: %v", err)
	}
	var risksPage struct {
		Items []model.DashboardRiskEvent `json:"items"`
		Total int64                      `json:"total"`
	}
	if err := json.Unmarshal(risksEnvelope.Data, &risksPage); err != nil {
		t.Fatalf("decode risks payload: %v", err)
	}
	if risksPage.Total != 3 {
		t.Fatalf("risks total = %d, want 3", risksPage.Total)
	}
	if len(risksPage.Items) != 2 {
		t.Fatalf("risk items len = %d, want 2", len(risksPage.Items))
	}
	if risksPage.Items[0].Severity != model.RiskSeverityCritical || risksPage.Items[0].InstanceName != gamma.Name || risksPage.Items[0].TargetName != unreachableTarget.Name {
		t.Fatalf("first risk item = %+v, want critical gamma/unreachable target", risksPage.Items[0])
	}
	if risksPage.Items[1].Severity != model.RiskSeverityWarning || risksPage.Items[1].InstanceName != alpha.Name {
		t.Fatalf("second risk item = %+v, want warning alpha", risksPage.Items[1])
	}

	trendsResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/trends", nil, token)
	if trendsResponse.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/trends status = %d, want %d, body = %s", trendsResponse.Code, http.StatusOK, trendsResponse.Body.String())
	}
	var trendsEnvelope apiEnvelope
	if err := json.Unmarshal(trendsResponse.Body.Bytes(), &trendsEnvelope); err != nil {
		t.Fatalf("decode trends envelope: %v", err)
	}
	var trends model.DashboardTrends
	if err := json.Unmarshal(trendsEnvelope.Data, &trends); err != nil {
		t.Fatalf("decode trends payload: %v", err)
	}
	if len(trends.BackupResults) != 7 {
		t.Fatalf("len(trends.BackupResults) = %d, want 7", len(trends.BackupResults))
	}
	if trends.InstanceHealth.Safe+trends.InstanceHealth.Caution+trends.InstanceHealth.Risk+trends.InstanceHealth.Danger != 3 {
		t.Fatalf("instance health distribution = %+v, want sum=3", trends.InstanceHealth)
	}
	trendByDate := make(map[string]model.DailyBackupResult, len(trends.BackupResults))
	for _, item := range trends.BackupResults {
		trendByDate[item.Date] = item
	}
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
	if trendByDate[today].Success != 2 || trendByDate[today].Failed != 1 {
		t.Fatalf("today trend = %+v, want success=2 failed=1", trendByDate[today])
	}
	if trendByDate[yesterday].Success != 1 || trendByDate[yesterday].Failed != 1 {
		t.Fatalf("yesterday trend = %+v, want success=1 failed=1", trendByDate[yesterday])
	}

	focusResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/focus-instances", nil, token)
	if focusResponse.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/focus-instances status = %d, want %d, body = %s", focusResponse.Code, http.StatusOK, focusResponse.Body.String())
	}
	var focusEnvelope apiEnvelope
	if err := json.Unmarshal(focusResponse.Body.Bytes(), &focusEnvelope); err != nil {
		t.Fatalf("decode focus envelope: %v", err)
	}
	var focusPayload struct {
		Items []model.FocusInstance `json:"items"`
	}
	if err := json.Unmarshal(focusEnvelope.Data, &focusPayload); err != nil {
		t.Fatalf("decode focus payload: %v", err)
	}
	if len(focusPayload.Items) != 3 {
		t.Fatalf("focus item count = %d, want 3", len(focusPayload.Items))
	}
	if focusPayload.Items[0].ID != alpha.ID {
		t.Fatalf("first focus instance id = %d, want %d", focusPayload.Items[0].ID, alpha.ID)
	}
	for index := 1; index < len(focusPayload.Items); index++ {
		if focusPayload.Items[index-1].DRScore > focusPayload.Items[index].DRScore {
			t.Fatalf("focus items not sorted by dr_score ascending: %+v", focusPayload.Items)
		}
	}
	if focusPayload.Items[0].UnresolvedRisks != 2 {
		t.Fatalf("alpha unresolved risks = %d, want 2", focusPayload.Items[0].UnresolvedRisks)
	}
	if focusPayload.Items[0].LastBackupStatus != "failed" {
		t.Fatalf("alpha last backup status = %q, want failed", focusPayload.Items[0].LastBackupStatus)
	}

	upcomingResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/dashboard/upcoming-tasks", nil, token)
	if upcomingResponse.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/upcoming-tasks status = %d, want %d, body = %s", upcomingResponse.Code, http.StatusOK, upcomingResponse.Body.String())
	}
	var upcomingEnvelope apiEnvelope
	if err := json.Unmarshal(upcomingResponse.Body.Bytes(), &upcomingEnvelope); err != nil {
		t.Fatalf("decode upcoming envelope: %v", err)
	}
	var upcomingPayload struct {
		Items []engine.UpcomingTask `json:"items"`
	}
	if err := json.Unmarshal(upcomingEnvelope.Data, &upcomingPayload); err != nil {
		t.Fatalf("decode upcoming payload: %v", err)
	}
	if len(upcomingPayload.Items) != 3 {
		t.Fatalf("upcoming item count = %d, want 3", len(upcomingPayload.Items))
	}
	if upcomingPayload.Items[0].PolicyID != alphaPolicy.ID || !upcomingPayload.Items[0].NextRunAt.Equal(now) {
		t.Fatalf("first upcoming item = %+v, want alpha policy at now", upcomingPayload.Items[0])
	}
	if upcomingPayload.Items[1].PolicyID != betaPolicy.ID || !upcomingPayload.Items[1].NextRunAt.Equal(now.Add(90*time.Minute)) {
		t.Fatalf("second upcoming item = %+v, want beta policy at %s", upcomingPayload.Items[1], now.Add(90*time.Minute).Format(time.RFC3339))
	}
	if upcomingPayload.Items[2].PolicyID != gammaPolicy.ID || !upcomingPayload.Items[2].NextRunAt.Equal(now.Add(165*time.Minute)) {
		t.Fatalf("third upcoming item = %+v, want gamma policy at %s", upcomingPayload.Items[2], now.Add(165*time.Minute).Format(time.RFC3339))
	}
}

func createDashboardTestInstance(t *testing.T, db *store.DB, name string) *model.Instance {
	t.Helper()

	instance := &model.Instance{Name: name, SourceType: "local", SourcePath: "/srv/" + name, Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance(%q) error = %v", name, err)
	}

	return instance
}

func createDashboardTestTarget(t *testing.T, db *store.DB, name, healthStatus string) *model.BackupTarget {
	t.Helper()

	target := &model.BackupTarget{
		Name:          name,
		BackupType:    "rolling",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  healthStatus,
		HealthMessage: healthStatus,
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget(%q) error = %v", name, err)
	}

	return target
}

func createDashboardTestPolicy(t *testing.T, db *store.DB, instanceID, targetID int64, name, interval string) *model.Policy {
	t.Helper()

	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           name,
		Type:           "rolling",
		TargetID:       targetID,
		ScheduleType:   "interval",
		ScheduleValue:  interval,
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 7,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy(%q) error = %v", name, err)
	}

	return policy
}

func createDashboardTestBackup(t *testing.T, db *store.DB, instanceID, policyID int64, status string, completedAt time.Time) *model.Backup {
	t.Helper()

	startedAt := completedAt.Add(-5 * time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policyID,
		TriggerSource:   model.BackupTriggerSourceScheduled,
		Type:            "rolling",
		Status:          status,
		SnapshotPath:    "/snapshots/" + status,
		BackupSizeBytes: 1024,
		ActualSizeBytes: 768,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: int64(completedAt.Sub(startedAt).Seconds()),
		ErrorMessage:    "",
		RsyncStats:      "{}",
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup(instance=%d policy=%d status=%s) error = %v", instanceID, policyID, status, err)
	}

	return backup
}

func createDashboardTestTask(t *testing.T, db *store.DB, instanceID int64, taskType, status string) *model.Task {
	t.Helper()

	startedAt := time.Now().UTC().Add(-5 * time.Minute)
	task := &model.Task{
		InstanceID:  instanceID,
		Type:        taskType,
		Status:      status,
		Progress:    0,
		CurrentStep: status,
		StartedAt:   &startedAt,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask(instance=%d status=%s) error = %v", instanceID, status, err)
	}

	return task
}

func createDashboardTestRiskEvent(t *testing.T, db *store.DB, instanceID *int64, targetID *int64, severity, source, message string, resolved bool, createdAt time.Time) {
	t.Helper()

	var resolvedAt any
	if resolved {
		resolvedAt = createdAt.Add(5 * time.Minute).Format(time.RFC3339Nano)
	}
	if _, err := db.Exec(
		`INSERT INTO risk_events (instance_id, target_id, severity, source, message, resolved, created_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		instanceID,
		targetID,
		severity,
		source,
		message,
		resolved,
		createdAt.Format(time.RFC3339Nano),
		resolvedAt,
	); err != nil {
		t.Fatalf("insert risk event %q error = %v", source, err)
	}
}

func mustDashboardScore(t *testing.T, disasterRecovery *servicepkg.DisasterRecoveryService, instanceID int64) *servicepkg.DisasterRecoveryScore {
	t.Helper()

	score, err := disasterRecovery.GetScore(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("GetScore(%d) error = %v", instanceID, err)
	}

	return score
}

func countAbnormalDashboardScores(scores ...float64) int {
	count := 0
	for _, score := range scores {
		if score < 70 {
			count++
		}
	}
	return count
}
