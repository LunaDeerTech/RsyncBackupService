package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const dashboardFocusInstanceLimit = 8

type dashboardScoredInstance struct {
	instance         model.Instance
	score            float64
	level            string
	unresolvedRisks  int
	lastBackupTime   *time.Time
	lastBackupStatus string
}

func (h *Handler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.disasterRecovery == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "dashboard service unavailable")
		return
	}

	runningTasks, err := h.db.CountTasksByStatus("running")
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count running tasks")
		return
	}
	queuedTasks, err := h.db.CountTasksByStatus("queued")
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count queued tasks")
		return
	}
	unresolvedRisks, err := h.db.CountUnresolvedRiskEvents()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count unresolved risks")
		return
	}
	totalBackups, err := h.db.CountBackups()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count backups")
		return
	}
	targets, err := h.db.ListBackupTargets()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list backup targets")
		return
	}
	scoredInstances, err := h.loadDashboardScoredInstances(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to calculate disaster recovery score")
		return
	}

	overview := model.DashboardOverview{
		RunningTasks:    runningTasks,
		QueuedTasks:     queuedTasks,
		UnresolvedRisks: unresolvedRisks,
		TotalInstances:  len(scoredInstances),
		TotalBackups:    totalBackups,
	}
	for _, target := range targets {
		switch target.HealthStatus {
		case "healthy":
			overview.TargetHealthSummary.Healthy++
		case "unreachable":
			overview.TargetHealthSummary.Unreachable++
		default:
			overview.TargetHealthSummary.Degraded++
		}
	}
	if len(scoredInstances) > 0 {
		totalScore := 0.0
		for _, item := range scoredInstances {
			totalScore += item.score
			if item.score < 70 {
				overview.AbnormalInstances++
			}
		}
		overview.SystemDRScore = totalScore / float64(len(scoredInstances))
	}
	overview.SystemDRLevel = service.DRLevelForScore(overview.SystemDRScore)

	JSON(w, http.StatusOK, overview)
}

func (h *Handler) ListDashboardRisks(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	pagination := ParsePagination(r)
	resolved := false
	items, total, err := h.db.ListDashboardRiskEvents(store.RiskEventQuery{
		Resolved: &resolved,
		Limit:    pagination.PageSize,
		Offset:   (pagination.Page - 1) * pagination.PageSize,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list dashboard risks")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(total, pagination.PageSize),
	})
}

func (h *Handler) GetDashboardTrends(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.disasterRecovery == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "dashboard service unavailable")
		return
	}

	backupResults, err := h.db.ListDailyBackupResults(7)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query backup trends")
		return
	}
	scoredInstances, err := h.loadDashboardScoredInstances(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to calculate instance health distribution")
		return
	}

	trends := model.DashboardTrends{BackupResults: backupResults}
	for _, item := range scoredInstances {
		switch item.level {
		case "safe":
			trends.InstanceHealth.Safe++
		case "caution":
			trends.InstanceHealth.Caution++
		case "risk":
			trends.InstanceHealth.Risk++
		default:
			trends.InstanceHealth.Danger++
		}
	}

	JSON(w, http.StatusOK, trends)
}

func (h *Handler) ListDashboardFocusInstances(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.disasterRecovery == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "dashboard service unavailable")
		return
	}

	scoredInstances, err := h.loadDashboardScoredInstances(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to calculate focus instances")
		return
	}

	sort.Slice(scoredInstances, func(i, j int) bool {
		left := scoredInstances[i]
		right := scoredInstances[j]
		if left.score != right.score {
			return left.score < right.score
		}
		if left.unresolvedRisks != right.unresolvedRisks {
			return left.unresolvedRisks > right.unresolvedRisks
		}
		return left.instance.ID < right.instance.ID
	})

	limit := dashboardFocusInstanceLimit
	if len(scoredInstances) < limit {
		limit = len(scoredInstances)
	}
	items := make([]model.FocusInstance, 0, limit)
	for _, item := range scoredInstances[:limit] {
		items = append(items, model.FocusInstance{
			ID:               item.instance.ID,
			Name:             item.instance.Name,
			DRScore:          item.score,
			DRLevel:          item.level,
			UnresolvedRisks:  item.unresolvedRisks,
			LastBackupTime:   item.lastBackupTime,
			LastBackupStatus: item.lastBackupStatus,
		})
	}

	JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListDashboardUpcomingTasks(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "scheduler unavailable")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"items": h.scheduler.GetUpcomingTasks(24 * time.Hour)})
}

func (h *Handler) loadDashboardScoredInstances(ctx context.Context) ([]dashboardScoredInstance, error) {
	instances, err := h.db.ListInstances()
	if err != nil {
		return nil, err
	}
	riskCounts, err := h.db.CountUnresolvedRiskEventsByInstance()
	if err != nil {
		return nil, err
	}
	scored := make([]dashboardScoredInstance, 0, len(instances))
	for _, instance := range instances {
		score, err := h.disasterRecovery.GetScore(ctx, instance.ID)
		if err != nil {
			return nil, err
		}
		lastBackupTime, lastBackupStatus, err := h.dashboardLastBackup(instance.ID)
		if err != nil {
			return nil, err
		}
		scored = append(scored, dashboardScoredInstance{
			instance:         instance,
			score:            score.Total,
			level:            score.Level,
			unresolvedRisks:  riskCounts[instance.ID],
			lastBackupTime:   lastBackupTime,
			lastBackupStatus: lastBackupStatus,
		})
	}

	return scored, nil
}

func (h *Handler) dashboardLastBackup(instanceID int64) (*time.Time, string, error) {
	lastBackup, err := h.db.GetLastBackup(instanceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	occurredAt := lastBackup.CreatedAt.UTC()
	if lastBackup.CompletedAt != nil {
		occurredAt = lastBackup.CompletedAt.UTC()
	} else if lastBackup.StartedAt != nil {
		occurredAt = lastBackup.StartedAt.UTC()
	}

	return &occurredAt, lastBackup.Status, nil
}
