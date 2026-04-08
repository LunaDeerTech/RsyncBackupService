package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
)

const policyErrorNotFound = 40405

type policyRequest struct {
	InstanceID     *int64 `json:"instance_id,omitempty"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	TargetID       int64  `json:"target_id"`
	ScheduleType   string `json:"schedule_type"`
	ScheduleValue  string `json:"schedule_value"`
	Enabled        bool   `json:"enabled"`
	Compression    bool   `json:"compression"`
	Encryption     bool   `json:"encryption"`
	EncryptionKey  string `json:"encryption_key,omitempty"`
	SplitEnabled   bool   `json:"split_enabled"`
	SplitSizeMB    *int   `json:"split_size_mb,omitempty"`
	RetentionType  string `json:"retention_type"`
	RetentionValue int    `json:"retention_value"`
}

type triggerPolicyRequest struct {
	EncryptionKey string `json:"encryption_key,omitempty"`
}

type policyInput struct {
	InstanceID        int64
	Name              string
	Type              string
	TargetID          int64
	ScheduleType      string
	ScheduleValue     string
	Enabled           bool
	Compression       bool
	Encryption        bool
	EncryptionKeyHash *string
	SplitEnabled      bool
	SplitSizeMB       *int
	RetentionType     string
	RetentionValue    int
}

type policyResponse struct {
	ID                  int64      `json:"id"`
	InstanceID          int64      `json:"instance_id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	TargetID            int64      `json:"target_id"`
	ScheduleType        string     `json:"schedule_type"`
	ScheduleValue       string     `json:"schedule_value"`
	Enabled             bool       `json:"enabled"`
	Compression         bool       `json:"compression"`
	Encryption          bool       `json:"encryption"`
	SplitEnabled        bool       `json:"split_enabled"`
	SplitSizeMB         *int       `json:"split_size_mb,omitempty"`
	RetentionType       string     `json:"retention_type"`
	RetentionValue      int        `json:"retention_value"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastExecutionTime   *time.Time `json:"last_execution_time,omitempty"`
	LastExecutionStatus *string    `json:"last_execution_status,omitempty"`
	LatestBackupID      *int64     `json:"latest_backup_id,omitempty"`
}

func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	policies, err := h.db.ListPoliciesByInstance(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list policies")
		return
	}

	summaries, err := h.db.ListPolicyExecutionSummaries(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load policy execution summaries")
		return
	}

	response := make([]policyResponse, 0, len(policies))
	for _, policy := range policies {
		response = append(response, buildPolicyResponse(policy, summaries[policy.ID]))
	}

	JSON(w, http.StatusOK, map[string]any{"items": response})
}

func (h *Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	var request policyRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := h.normalizePolicyInput(instanceID, request, nil)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	policy := &model.Policy{
		InstanceID:        input.InstanceID,
		Name:              input.Name,
		Type:              input.Type,
		TargetID:          input.TargetID,
		ScheduleType:      input.ScheduleType,
		ScheduleValue:     input.ScheduleValue,
		Enabled:           input.Enabled,
		Compression:       input.Compression,
		Encryption:        input.Encryption,
		EncryptionKeyHash: cloneOptionalPolicyString(input.EncryptionKeyHash),
		SplitEnabled:      input.SplitEnabled,
		SplitSizeMB:       cloneOptionalInt(input.SplitSizeMB),
		RetentionType:     input.RetentionType,
		RetentionValue:    input.RetentionValue,
	}
	if err := h.db.CreatePolicy(policy); err != nil {
		writePolicyError(w, err, "failed to create policy")
		return
	}
	if h.disasterRecovery != nil {
		h.disasterRecovery.Invalidate(policy.InstanceID)
	}
	if policy.Enabled && h.scheduler != nil {
		h.scheduler.ReloadPolicy(policy.ID)
	}
	h.writeCurrentUserAudit(r, policy.InstanceID, audit.ActionPolicyCreate, map[string]any{
		"policy_id":       policy.ID,
		"instance_id":     policy.InstanceID,
		"name":            policy.Name,
		"type":            policy.Type,
		"target_id":       policy.TargetID,
		"schedule_type":   policy.ScheduleType,
		"schedule_value":  policy.ScheduleValue,
		"enabled":         policy.Enabled,
		"retention_type":  policy.RetentionType,
		"retention_value": policy.RetentionValue,
	})

	JSON(w, http.StatusCreated, buildPolicyResponse(*policy, model.PolicyExecutionSummary{}))
}

func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, policyID, err := policyRequestIDs(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	current, err := h.getPolicyForInstance(instanceID, policyID)
	if err != nil {
		writePolicyError(w, err, "failed to query policy")
		return
	}

	var request policyRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := h.normalizePolicyInput(instanceID, request, current)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	current.Name = input.Name
	current.Type = input.Type
	current.TargetID = input.TargetID
	current.ScheduleType = input.ScheduleType
	current.ScheduleValue = input.ScheduleValue
	current.Enabled = input.Enabled
	current.Compression = input.Compression
	current.Encryption = input.Encryption
	current.EncryptionKeyHash = cloneOptionalPolicyString(input.EncryptionKeyHash)
	current.SplitEnabled = input.SplitEnabled
	current.SplitSizeMB = cloneOptionalInt(input.SplitSizeMB)
	current.RetentionType = input.RetentionType
	current.RetentionValue = input.RetentionValue

	if err := h.db.UpdatePolicy(current); err != nil {
		writePolicyError(w, err, "failed to update policy")
		return
	}
	if h.disasterRecovery != nil {
		h.disasterRecovery.Invalidate(current.InstanceID)
	}
	if h.scheduler != nil {
		h.scheduler.ReloadPolicy(current.ID)
	}
	h.writeCurrentUserAudit(r, current.InstanceID, audit.ActionPolicyUpdate, map[string]any{
		"policy_id":       current.ID,
		"instance_id":     current.InstanceID,
		"name":            current.Name,
		"type":            current.Type,
		"target_id":       current.TargetID,
		"schedule_type":   current.ScheduleType,
		"schedule_value":  current.ScheduleValue,
		"enabled":         current.Enabled,
		"retention_type":  current.RetentionType,
		"retention_value": current.RetentionValue,
	})

	summary, err := h.loadPolicySummary(instanceID, current.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load policy execution summary")
		return
	}

	JSON(w, http.StatusOK, buildPolicyResponse(*current, summary))
}

func (h *Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, policyID, err := policyRequestIDs(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	policy, err := h.getPolicyForInstance(instanceID, policyID)
	if err != nil {
		writePolicyError(w, err, "failed to query policy")
		return
	}

	if err := h.db.DeletePolicy(policyID); err != nil {
		writePolicyError(w, err, "failed to delete policy")
		return
	}
	if h.disasterRecovery != nil {
		h.disasterRecovery.Invalidate(policy.InstanceID)
	}
	if h.scheduler != nil {
		h.scheduler.RemovePolicy(policyID)
	}
	h.writeCurrentUserAudit(r, instanceID, audit.ActionPolicyDelete, map[string]any{
		"deleted_policy_id": policy.ID,
		"instance_id":       policy.InstanceID,
		"name":              policy.Name,
		"type":              policy.Type,
		"target_id":         policy.TargetID,
	})

	JSON(w, http.StatusOK, map[string]string{"message": "policy deleted"})
}

func (h *Handler) TriggerPolicy(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, policyID, err := policyRequestIDs(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	policy, err := h.getPolicyForInstance(instanceID, policyID)
	if err != nil {
		writePolicyError(w, err, "failed to query policy")
		return
	}

	var request triggerPolicyRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	encryptionKey := strings.TrimSpace(request.EncryptionKey)
	if policy.Type == "cold" && policy.Encryption {
		if encryptionKey == "" {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "encryption_key is required when triggering encrypted cold policies")
			return
		}
		if policy.EncryptionKeyHash != nil && *policy.EncryptionKeyHash != "" && !authcrypto.ValidateEncryptionKey(encryptionKey, *policy.EncryptionKeyHash) {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "encryption_key does not match policy")
			return
		}
	}

	backup, task, err := h.db.CreatePendingPolicyRun(policy)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to create pending policy run")
		return
	}
	if h.taskQueue != nil {
		if policy.Type == "cold" && encryptionKey != "" {
			h.taskQueue.SetColdEncryptionKey(task.ID, encryptionKey)
		}
		if err := h.taskQueue.Enqueue(task); err != nil {
			Error(w, http.StatusInternalServerError, authErrorInternal, "failed to enqueue task")
			return
		}
	}
	h.writeCurrentUserAudit(r, instanceID, audit.ActionBackupTrigger, map[string]any{
		"backup_id":      backup.ID,
		"task_id":        task.ID,
		"policy_id":      policy.ID,
		"policy_name":    policy.Name,
		"type":           policy.Type,
		"trigger_source": backup.TriggerSource,
	})

	JSON(w, http.StatusCreated, map[string]any{"backup": backup, "task": task})
}

func (h *Handler) normalizePolicyInput(instanceID int64, request policyRequest, current *model.Policy) (policyInput, error) {
	if request.InstanceID != nil && *request.InstanceID != instanceID {
		return policyInput{}, fmt.Errorf("instance_id must match the request path")
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		return policyInput{}, fmt.Errorf("name is required")
	}

	policyType := strings.ToLower(strings.TrimSpace(request.Type))
	switch policyType {
	case "rolling", "cold":
	default:
		return policyInput{}, fmt.Errorf("type must be rolling or cold")
	}

	if request.TargetID <= 0 {
		return policyInput{}, fmt.Errorf("target_id must be positive")
	}

	target, err := h.db.GetBackupTargetByID(request.TargetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return policyInput{}, fmt.Errorf("target_id is invalid")
		}
		return policyInput{}, err
	}
	if target.BackupType != policyType {
		return policyInput{}, fmt.Errorf("policy type %s is incompatible with target backup_type %s", policyType, target.BackupType)
	}

	scheduleType := strings.ToLower(strings.TrimSpace(request.ScheduleType))
	scheduleValue := strings.TrimSpace(request.ScheduleValue)
	switch scheduleType {
	case "cron":
		if err := validateCronExpression(scheduleValue); err != nil {
			return policyInput{}, err
		}
	case "interval":
		seconds, err := strconv.Atoi(scheduleValue)
		if err != nil || seconds <= 0 {
			return policyInput{}, fmt.Errorf("schedule_value must be a positive integer when schedule_type is interval")
		}
	default:
		return policyInput{}, fmt.Errorf("schedule_type must be interval or cron")
	}

	retentionType := strings.ToLower(strings.TrimSpace(request.RetentionType))
	switch retentionType {
	case "time", "count":
	default:
		return policyInput{}, fmt.Errorf("retention_type must be time or count")
	}
	if request.RetentionValue <= 0 {
		return policyInput{}, fmt.Errorf("retention_value must be positive")
	}

	encryptionKey := strings.TrimSpace(request.EncryptionKey)
	input := policyInput{
		InstanceID:     instanceID,
		Name:           name,
		Type:           policyType,
		TargetID:       request.TargetID,
		ScheduleType:   scheduleType,
		ScheduleValue:  scheduleValue,
		Enabled:        request.Enabled,
		RetentionType:  retentionType,
		RetentionValue: request.RetentionValue,
	}

	if policyType == "rolling" {
		if request.Compression {
			return policyInput{}, fmt.Errorf("compression is only supported for cold policies")
		}
		if request.Encryption {
			return policyInput{}, fmt.Errorf("encryption is only supported for cold policies")
		}
		if encryptionKey != "" {
			return policyInput{}, fmt.Errorf("encryption_key is only supported for cold policies")
		}
		if request.SplitEnabled {
			return policyInput{}, fmt.Errorf("split_enabled is only supported for cold policies")
		}
		if request.SplitSizeMB != nil {
			return policyInput{}, fmt.Errorf("split_size_mb is only supported for cold policies")
		}
		return input, nil
	}

	input.Compression = request.Compression
	input.Encryption = request.Encryption
	input.SplitEnabled = request.SplitEnabled

	if request.Encryption {
		switch {
		case encryptionKey != "":
			hash := authcrypto.HashEncryptionKey(encryptionKey)
			input.EncryptionKeyHash = &hash
		case current != nil && current.Encryption && current.EncryptionKeyHash != nil:
			input.EncryptionKeyHash = cloneOptionalPolicyString(current.EncryptionKeyHash)
		default:
			return policyInput{}, fmt.Errorf("encryption_key is required when encryption is enabled")
		}
	} else {
		if encryptionKey != "" {
			return policyInput{}, fmt.Errorf("encryption_key is only supported when encryption is enabled")
		}
		input.EncryptionKeyHash = nil
	}

	if request.SplitEnabled {
		if request.SplitSizeMB == nil || *request.SplitSizeMB <= 0 {
			return policyInput{}, fmt.Errorf("split_size_mb must be positive when split_enabled is true")
		}
		input.SplitSizeMB = cloneOptionalInt(request.SplitSizeMB)
	} else {
		input.SplitSizeMB = nil
	}

	return input, nil
}

func buildPolicyResponse(policy model.Policy, summary model.PolicyExecutionSummary) policyResponse {
	return policyResponse{
		ID:                  policy.ID,
		InstanceID:          policy.InstanceID,
		Name:                policy.Name,
		Type:                policy.Type,
		TargetID:            policy.TargetID,
		ScheduleType:        policy.ScheduleType,
		ScheduleValue:       policy.ScheduleValue,
		Enabled:             policy.Enabled,
		Compression:         policy.Compression,
		Encryption:          policy.Encryption,
		SplitEnabled:        policy.SplitEnabled,
		SplitSizeMB:         cloneOptionalInt(policy.SplitSizeMB),
		RetentionType:       policy.RetentionType,
		RetentionValue:      policy.RetentionValue,
		CreatedAt:           policy.CreatedAt,
		UpdatedAt:           policy.UpdatedAt,
		LastExecutionTime:   cloneOptionalTime(summary.LastExecutionTime),
		LastExecutionStatus: cloneOptionalPolicyString(summary.LastExecutionStatus),
		LatestBackupID:      cloneOptionalInt64(summary.LatestBackupID),
	}
}

func (h *Handler) getPolicyForInstance(instanceID, policyID int64) (*model.Policy, error) {
	policy, err := h.db.GetPolicyByID(policyID)
	if err != nil {
		return nil, err
	}
	if policy.InstanceID != instanceID {
		return nil, sql.ErrNoRows
	}

	return policy, nil
}

func (h *Handler) loadPolicySummary(instanceID, policyID int64) (model.PolicyExecutionSummary, error) {
	summaries, err := h.db.ListPolicyExecutionSummaries(instanceID)
	if err != nil {
		return model.PolicyExecutionSummary{}, err
	}

	return summaries[policyID], nil
}

func writePolicyError(w http.ResponseWriter, err error, defaultMessage string) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		Error(w, http.StatusNotFound, policyErrorNotFound, "policy not found")
	default:
		Error(w, http.StatusInternalServerError, authErrorInternal, defaultMessage)
	}
}

func policyRequestIDs(r *http.Request) (int64, int64, error) {
	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid instance id")
	}

	policyID, err := policyIDFromRequest(r)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid policy id")
	}

	return instanceID, policyID, nil
}

func policyIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("pid"))
	if rawID == "" {
		return 0, fmt.Errorf("policy id is required")
	}

	policyID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse policy id %q: %w", rawID, err)
	}
	if policyID <= 0 {
		return 0, fmt.Errorf("policy id must be positive")
	}

	return policyID, nil
}

func validateCronExpression(expression string) error {
	if _, err := engine.ParseCron(expression); err != nil {
		return err
	}
	return nil
}

func validateCronField(field string, min, max int) error {
	for _, part := range strings.Split(field, ",") {
		segment := strings.TrimSpace(part)
		if segment == "" {
			return fmt.Errorf("empty field segment")
		}
		if err := validateCronSegment(segment, min, max); err != nil {
			return err
		}
	}

	return nil
}

func validateCronSegment(segment string, min, max int) error {
	base := segment
	if strings.Contains(segment, "/") {
		left, right, found := strings.Cut(segment, "/")
		if !found || strings.TrimSpace(right) == "" {
			return fmt.Errorf("invalid step segment %q", segment)
		}
		step, err := strconv.Atoi(strings.TrimSpace(right))
		if err != nil || step <= 0 {
			return fmt.Errorf("invalid step value %q", right)
		}
		base = strings.TrimSpace(left)
	}

	if base == "*" {
		return nil
	}
	if strings.Contains(base, "-") {
		startRaw, endRaw, found := strings.Cut(base, "-")
		if !found {
			return fmt.Errorf("invalid range segment %q", base)
		}
		start, err := strconv.Atoi(strings.TrimSpace(startRaw))
		if err != nil {
			return fmt.Errorf("invalid range start %q", startRaw)
		}
		end, err := strconv.Atoi(strings.TrimSpace(endRaw))
		if err != nil {
			return fmt.Errorf("invalid range end %q", endRaw)
		}
		if start < min || end > max || start > end {
			return fmt.Errorf("range %q is out of bounds", base)
		}
		return nil
	}

	value, err := strconv.Atoi(base)
	if err != nil {
		return fmt.Errorf("invalid field value %q", base)
	}
	if value < min || value > max {
		return fmt.Errorf("value %q is out of bounds", base)
	}

	return nil
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func cloneOptionalPolicyString(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func cloneOptionalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
