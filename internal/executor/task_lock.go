package executor

import (
	"errors"
	"fmt"
)

var (
	ErrTaskConflict = errors.New("task conflict")
	ErrTaskNotFound = errors.New("running task not found")
)

func BuildTaskLockKey(instanceID, storageTargetID uint) string {
	return fmt.Sprintf("instance:%d:target:%d", instanceID, storageTargetID)
}

func ParseTaskLockKey(lockKey string) (uint, uint, error) {
	var instanceID uint
	var storageTargetID uint

	if _, err := fmt.Sscanf(lockKey, "instance:%d:target:%d", &instanceID, &storageTargetID); err != nil {
		return 0, 0, fmt.Errorf("parse task lock key %q: %w", lockKey, err)
	}

	return instanceID, storageTargetID, nil
}

func NewTaskConflictError(lockKey string) error {
	return fmt.Errorf("%w: %s", ErrTaskConflict, lockKey)
}
