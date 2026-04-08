package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

const drCacheTTL = 5 * time.Minute

type DisasterRecoveryScore struct {
	Total          float64   `json:"total"`
	Level          string    `json:"level"`
	Freshness      float64   `json:"freshness"`
	RecoveryPoints float64   `json:"recovery_points"`
	Redundancy     float64   `json:"redundancy"`
	Stability      float64   `json:"stability"`
	Deductions     []string  `json:"deductions"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type DRCalculator struct {
	db  *store.DB
	now func() time.Time
}

type DRCache struct {
	mu    sync.RWMutex
	cache map[int64]*cachedScore
	now   func() time.Time
	ttl   time.Duration
}

type cachedScore struct {
	score     *DisasterRecoveryScore
	expiresAt time.Time
}

type DisasterRecoveryService struct {
	calculator *DRCalculator
	cache      *DRCache
}

func NewDRCalculator(db *store.DB) *DRCalculator {
	return &DRCalculator{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func NewDRCache() *DRCache {
	return &DRCache{
		cache: make(map[int64]*cachedScore),
		now:   func() time.Time { return time.Now().UTC() },
		ttl:   drCacheTTL,
	}
}

func NewDisasterRecoveryService(db *store.DB) *DisasterRecoveryService {
	return &DisasterRecoveryService{
		calculator: NewDRCalculator(db),
		cache:      NewDRCache(),
	}
}

func (s *DisasterRecoveryService) SetClock(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	if s.calculator != nil {
		s.calculator.now = func() time.Time { return now().UTC() }
	}
	if s.cache != nil {
		s.cache.now = func() time.Time { return now().UTC() }
	}
}

func (c *DRCache) Get(instanceID int64) (*DisasterRecoveryScore, bool) {
	if c == nil || instanceID <= 0 {
		return nil, false
	}

	now := c.now().UTC()
	c.mu.RLock()
	entry, ok := c.cache[instanceID]
	c.mu.RUnlock()
	if !ok || entry == nil || entry.score == nil || !entry.expiresAt.After(now) {
		if ok {
			c.mu.Lock()
			delete(c.cache, instanceID)
			c.mu.Unlock()
		}
		return nil, false
	}

	return cloneDisasterRecoveryScore(entry.score), true
}

func (c *DRCache) Set(instanceID int64, score *DisasterRecoveryScore) {
	if c == nil || instanceID <= 0 || score == nil {
		return
	}

	c.mu.Lock()
	c.cache[instanceID] = &cachedScore{
		score:     cloneDisasterRecoveryScore(score),
		expiresAt: c.now().UTC().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *DRCache) Invalidate(instanceID int64) {
	if c == nil || instanceID <= 0 {
		return
	}

	c.mu.Lock()
	delete(c.cache, instanceID)
	c.mu.Unlock()
}

func (s *DisasterRecoveryService) GetScore(ctx context.Context, instanceID int64) (*DisasterRecoveryScore, error) {
	if s == nil || s.calculator == nil || s.cache == nil {
		return nil, fmt.Errorf("disaster recovery service unavailable")
	}
	if cached, ok := s.cache.Get(instanceID); ok {
		return cached, nil
	}

	score, err := s.calculator.Calculate(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	s.cache.Set(instanceID, score)
	return cloneDisasterRecoveryScore(score), nil
}

func (s *DisasterRecoveryService) Invalidate(instanceID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.Invalidate(instanceID)
}

func (s *DisasterRecoveryService) Cache() *DRCache {
	if s == nil {
		return nil
	}
	return s.cache
}

func (s *DisasterRecoveryService) InvalidateByTarget(targetID int64) {
	if s == nil || s.calculator == nil || s.calculator.db == nil || s.cache == nil || targetID <= 0 {
		return
	}

	instanceIDs, err := s.calculator.db.ListInstanceIDsByTargetID(targetID)
	if err != nil {
		return
	}
	for _, instanceID := range instanceIDs {
		s.cache.Invalidate(instanceID)
	}
}

func (c *DRCalculator) Calculate(ctx context.Context, instanceID int64) (*DisasterRecoveryScore, error) {
	if c == nil || c.db == nil {
		return nil, fmt.Errorf("disaster recovery calculator unavailable")
	}
	if instanceID <= 0 {
		return nil, fmt.Errorf("instance id must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	policies, err := c.db.ListPoliciesByInstance(instanceID)
	if err != nil {
		return nil, err
	}
	enabledPolicies := make([]model.Policy, 0, len(policies))
	for _, policy := range policies {
		if policy.Enabled {
			enabledPolicies = append(enabledPolicies, policy)
		}
	}

	now := c.now().UTC()
	if len(enabledPolicies) == 0 {
		return &DisasterRecoveryScore{
			Total:          0,
			Level:          scoreToLevel(0),
			Freshness:      0,
			RecoveryPoints: 0,
			Redundancy:     0,
			Stability:      0,
			Deductions:     []string{"未启用任何备份策略"},
			CalculatedAt:   now,
		}, nil
	}

	freshness, freshnessReasons, err := c.calculateFreshness(instanceID, enabledPolicies, now)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	recoveryPoints, recoveryReasons, err := c.calculateRecoveryPoints(enabledPolicies, now)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	redundancy, redundancyReasons, targets, err := c.calculateRedundancy(enabledPolicies)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	stability, stabilityReasons, err := c.calculateStability(instanceID, targets)
	if err != nil {
		return nil, err
	}

	freshness = roundScore(freshness)
	recoveryPoints = roundScore(recoveryPoints)
	redundancy = roundScore(redundancy)
	stability = roundScore(stability)
	total := roundScore(0.35*freshness + 0.30*recoveryPoints + 0.20*redundancy + 0.15*stability)

	return &DisasterRecoveryScore{
		Total:          total,
		Level:          scoreToLevel(total),
		Freshness:      freshness,
		RecoveryPoints: recoveryPoints,
		Redundancy:     redundancy,
		Stability:      stability,
		Deductions:     mergeUniqueStrings(freshnessReasons, recoveryReasons, redundancyReasons, stabilityReasons),
		CalculatedAt:   now,
	}, nil
}

func (c *DRCalculator) calculateFreshness(instanceID int64, policies []model.Policy, now time.Time) (float64, []string, error) {
	periods := make([]time.Duration, 0, len(policies))
	policyIDs := make([]int64, 0, len(policies))
	reasons := make([]string, 0)
	for _, policy := range policies {
		period, err := policySchedulePeriod(policy, now)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("策略 %s 调度周期无效", policy.Name))
			continue
		}
		periods = append(periods, period)
		policyIDs = append(policyIDs, policy.ID)
	}
	if len(periods) == 0 {
		return 0, append(reasons, "未找到有效的自动备份计划"), nil
	}

	shortest := periods[0]
	for _, period := range periods[1:] {
		if period < shortest {
			shortest = period
		}
	}

	latestBackup, err := c.db.GetLatestSuccessfulBackupByPolicies(instanceID, policyIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, append(reasons, "没有成功的备份记录"), nil
		}
		return 0, nil, err
	}

	elapsed := backupOccurredAt(latestBackup).Sub(now)
	if elapsed < 0 {
		elapsed = -elapsed
	}
	switch {
	case elapsed <= shortest:
		return 100, reasons, nil
	case elapsed <= 2*shortest:
		ratio := float64(elapsed-shortest) / float64(shortest)
		return 100 - 40*ratio, reasons, nil
	case elapsed <= 3*shortest:
		ratio := float64(elapsed-2*shortest) / float64(shortest)
		return 60 - 30*ratio, append(reasons, "最近成功备份已超过两个周期"), nil
	default:
		return 20, append(reasons, "最近成功备份严重滞后"), nil
	}
}

func (c *DRCalculator) calculateRecoveryPoints(policies []model.Policy, now time.Time) (float64, []string, error) {
	total := 0.0
	reasons := make([]string, 0)
	for _, policy := range policies {
		count, err := c.countAvailableRecoveryPoints(policy, now)
		if err != nil {
			return 0, nil, err
		}

		score := 0.0
		switch {
		case count <= 0:
			reasons = append(reasons, fmt.Sprintf("策略 %s 没有可用恢复点", policy.Name))
		case count == 1:
			score = 80
		default:
			score = math.Min(100, 80+float64(count-1)*10)
		}
		total += score
	}

	return total / float64(len(policies)), reasons, nil
}

func (c *DRCalculator) countAvailableRecoveryPoints(policy model.Policy, now time.Time) (int, error) {
	switch strings.ToLower(strings.TrimSpace(policy.RetentionType)) {
	case "count":
		backups, err := c.db.ListSuccessfulBackupsByPolicy(policy.ID, policy.RetentionValue)
		if err != nil {
			return 0, err
		}
		return len(backups), nil
	case "time":
		since := now.AddDate(0, 0, -policy.RetentionValue)
		backups, err := c.db.ListSuccessfulBackupsByPolicySince(policy.ID, since)
		if err != nil {
			return 0, err
		}
		return len(backups), nil
	default:
		return 0, nil
	}
}

func (c *DRCalculator) calculateRedundancy(policies []model.Policy) (float64, []string, map[int64]*model.BackupTarget, error) {
	targets := make(map[int64]*model.BackupTarget)
	reasons := make([]string, 0)
	for _, policy := range policies {
		if _, exists := targets[policy.TargetID]; exists {
			continue
		}
		target, err := c.db.GetBackupTargetByID(policy.TargetID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				reasons = append(reasons, fmt.Sprintf("策略 %s 引用的目标不存在", policy.Name))
				continue
			}
			return 0, nil, nil, err
		}
		targets[target.ID] = target
	}

	if len(targets) == 0 {
		return 0, append(reasons, "没有可用的备份目标"), targets, nil
	}

	healthyCount := 0
	localHealthy := 0
	sshHealthy := 0
	unhealthyCount := 0
	for _, target := range targets {
		if strings.EqualFold(target.HealthStatus, "healthy") {
			healthyCount++
			switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
			case "local":
				localHealthy++
			case "ssh":
				sshHealthy++
			}
			continue
		}
		unhealthyCount++
		reasons = append(reasons, fmt.Sprintf("目标 %s 当前状态异常", target.Name))
	}

	base := 0.0
	switch {
	case healthyCount >= 2 && sshHealthy >= 1:
		base = 100
	case healthyCount >= 2:
		base = 70
	case healthyCount == 1 && sshHealthy >= 1:
		base = 60
	case healthyCount == 1 && localHealthy >= 1:
		base = 40
	case healthyCount == 1:
		base = 50
	default:
		base = 0
	}

	score := base - float64(unhealthyCount*20)
	if score < 0 {
		score = 0
	}
	return score, reasons, targets, nil
}

func (c *DRCalculator) calculateStability(instanceID int64, targets map[int64]*model.BackupTarget) (float64, []string, error) {
	blocking, blockingReasons, err := c.detectBlockingRisk(instanceID, targets)
	if err != nil {
		return 0, nil, err
	}
	if blocking {
		return 0, blockingReasons, nil
	}

	backups, err := c.db.ListRecentBackupsByInstanceAllStatuses(instanceID, 10)
	if err != nil {
		return 0, nil, err
	}
	if len(backups) == 0 {
		return 20, []string{"近期没有备份执行记录"}, nil
	}

	successCount := 0
	consecutiveFailures := 0
	maxConsecutiveFailures := 0
	for _, backup := range backups {
		if strings.EqualFold(backup.Status, "success") {
			successCount++
			consecutiveFailures = 0
			continue
		}
		consecutiveFailures++
		if consecutiveFailures > maxConsecutiveFailures {
			maxConsecutiveFailures = consecutiveFailures
		}
	}

	score := (float64(successCount) / float64(len(backups))) * 80
	score += 20
	reasons := make([]string, 0)
	if maxConsecutiveFailures >= 3 {
		score = math.Min(score, 15)
		reasons = append(reasons, "最近存在连续失败记录")
	}

	return score, reasons, nil
}

func (c *DRCalculator) detectBlockingRisk(instanceID int64, targets map[int64]*model.BackupTarget) (bool, []string, error) {
	reasons := make([]string, 0)
	for _, target := range targets {
		message := strings.ToLower(strings.TrimSpace(target.HealthMessage))
		status := strings.ToLower(strings.TrimSpace(target.HealthStatus))
		switch {
		case status == "unreachable":
			reasons = append(reasons, fmt.Sprintf("目标 %s 不可达", target.Name))
		case target.TotalCapacityBytes != nil && target.UsedCapacityBytes != nil && *target.TotalCapacityBytes > 0 && *target.UsedCapacityBytes >= *target.TotalCapacityBytes:
			reasons = append(reasons, fmt.Sprintf("目标 %s 容量耗尽", target.Name))
		case strings.Contains(message, "capacity") || strings.Contains(message, "disk full") || strings.Contains(message, "no space"):
			reasons = append(reasons, fmt.Sprintf("目标 %s 容量不足", target.Name))
		case strings.Contains(message, "credential") || strings.Contains(message, "auth") || strings.Contains(message, "permission") || strings.Contains(message, "private key"):
			reasons = append(reasons, fmt.Sprintf("目标 %s 凭证异常", target.Name))
		}
	}

	riskMessages, err := c.db.ListUnresolvedRiskEventMessagesByInstance(instanceID)
	if err != nil {
		return false, nil, err
	}
	for _, message := range riskMessages {
		lower := strings.ToLower(strings.TrimSpace(message))
		if strings.Contains(lower, "unreachable") || strings.Contains(lower, "capacity") || strings.Contains(lower, "credential") || strings.Contains(lower, "auth") || strings.Contains(lower, "不可达") || strings.Contains(lower, "容量") || strings.Contains(lower, "凭证") {
			reasons = append(reasons, strings.TrimSpace(message))
		}
	}

	return len(reasons) > 0, mergeUniqueStrings(reasons), nil
}

func scoreToLevel(total float64) string {
	switch {
	case total >= 85:
		return "safe"
	case total >= 70:
		return "caution"
	case total >= 40:
		return "risk"
	default:
		return "danger"
	}
}

func DRLevelForScore(total float64) string {
	return scoreToLevel(total)
}

func cloneDisasterRecoveryScore(score *DisasterRecoveryScore) *DisasterRecoveryScore {
	if score == nil {
		return nil
	}
	cloned := *score
	if score.Deductions != nil {
		cloned.Deductions = append([]string(nil), score.Deductions...)
	}
	return &cloned
}

func mergeUniqueStrings(groups ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			if _, exists := seen[trimmed]; exists {
				continue
			}
			seen[trimmed] = struct{}{}
			merged = append(merged, trimmed)
		}
	}
	return merged
}

func roundScore(value float64) float64 {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return math.Round(value*10) / 10
}

func backupOccurredAt(backup *model.Backup) time.Time {
	if backup == nil {
		return time.Time{}
	}
	if backup.CompletedAt != nil {
		return backup.CompletedAt.UTC()
	}
	if backup.StartedAt != nil {
		return backup.StartedAt.UTC()
	}
	return backup.CreatedAt.UTC()
}

func policySchedulePeriod(policy model.Policy, now time.Time) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(policy.ScheduleType)) {
	case "interval":
		seconds, err := strconv.Atoi(strings.TrimSpace(policy.ScheduleValue))
		if err != nil || seconds <= 0 {
			return 0, fmt.Errorf("invalid interval %q", policy.ScheduleValue)
		}
		return time.Duration(seconds) * time.Second, nil
	case "cron":
		next, err := nextCronTime(strings.TrimSpace(policy.ScheduleValue), now)
		if err != nil {
			return 0, err
		}
		period := next.Sub(now)
		if period <= 0 {
			return 0, fmt.Errorf("cron expression %q produced non-positive period", policy.ScheduleValue)
		}
		return period, nil
	default:
		return 0, fmt.Errorf("unsupported schedule type %q", policy.ScheduleType)
	}
}

func nextCronTime(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("schedule_value must be a standard 5-field cron expression")
	}

	minutes, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return time.Time{}, err
	}
	hours, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return time.Time{}, err
	}
	days, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return time.Time{}, err
	}
	months, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return time.Time{}, err
	}
	weekdays, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return time.Time{}, err
	}

	minuteSet := sliceToSet(minutes)
	hourSet := sliceToSet(hours)
	daySet := sliceToSet(days)
	monthSet := sliceToSet(months)
	weekdaySet := sliceToSet(weekdays)
	allDays := len(days) == 31
	allWeekdays := len(weekdays) == 7

	candidate := from.Truncate(time.Minute).Add(time.Minute)
	deadline := candidate.AddDate(5, 0, 0)
	for !candidate.After(deadline) {
		if cronMatches(candidate, minuteSet, hourSet, daySet, monthSet, weekdaySet, allDays, allWeekdays) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("cron expression %q has no next run", expr)
}

func cronMatches(ts time.Time, minuteSet, hourSet, daySet, monthSet, weekdaySet map[int]struct{}, allDays, allWeekdays bool) bool {
	if _, ok := minuteSet[ts.Minute()]; !ok {
		return false
	}
	if _, ok := hourSet[ts.Hour()]; !ok {
		return false
	}
	if _, ok := monthSet[int(ts.Month())]; !ok {
		return false
	}
	_, dayMatch := daySet[ts.Day()]
	_, weekdayMatch := weekdaySet[int(ts.Weekday())]
	switch {
	case allDays && allWeekdays:
		return true
	case allDays:
		return weekdayMatch
	case allWeekdays:
		return dayMatch
	default:
		return dayMatch || weekdayMatch
	}
}

func parseCronField(field string, min, max int, normalizeWeekday bool) ([]int, error) {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" {
		return nil, fmt.Errorf("empty cron field")
	}

	values := make(map[int]struct{})
	for _, part := range strings.Split(trimmed, ",") {
		segment := strings.TrimSpace(part)
		if segment == "" {
			return nil, fmt.Errorf("empty cron field segment")
		}
		expanded, err := expandCronSegment(segment, min, max, normalizeWeekday)
		if err != nil {
			return nil, err
		}
		for _, value := range expanded {
			values[value] = struct{}{}
		}
	}

	result := make([]int, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Ints(result)
	return result, nil
}

func expandCronSegment(segment string, min, max int, normalizeWeekday bool) ([]int, error) {
	base := strings.TrimSpace(segment)
	step := 1
	if strings.Contains(base, "/") {
		left, right, found := strings.Cut(base, "/")
		if !found || strings.TrimSpace(right) == "" {
			return nil, fmt.Errorf("invalid step segment %q", segment)
		}
		parsedStep, err := strconv.Atoi(strings.TrimSpace(right))
		if err != nil || parsedStep <= 0 {
			return nil, fmt.Errorf("invalid step value %q", right)
		}
		base = strings.TrimSpace(left)
		step = parsedStep
	}

	start := min
	end := max
	switch {
	case base == "" || base == "*":
	case strings.Contains(base, "-"):
		left, right, found := strings.Cut(base, "-")
		if !found {
			return nil, fmt.Errorf("invalid range segment %q", base)
		}
		parsedStart, err := strconv.Atoi(strings.TrimSpace(left))
		if err != nil {
			return nil, fmt.Errorf("invalid range start %q", left)
		}
		parsedEnd, err := strconv.Atoi(strings.TrimSpace(right))
		if err != nil {
			return nil, fmt.Errorf("invalid range end %q", right)
		}
		start = parsedStart
		end = parsedEnd
	default:
		parsedValue, err := strconv.Atoi(base)
		if err != nil {
			return nil, fmt.Errorf("invalid field value %q", base)
		}
		start = parsedValue
		end = parsedValue
	}

	if start < min || end > max || start > end {
		return nil, fmt.Errorf("range %q is out of bounds", base)
	}

	values := make([]int, 0, ((end-start)/step)+1)
	for value := start; value <= end; value += step {
		normalized := value
		if normalizeWeekday && value == 7 {
			normalized = 0
		}
		values = append(values, normalized)
	}

	return values, nil
}

func sliceToSet(values []int) map[int]struct{} {
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
