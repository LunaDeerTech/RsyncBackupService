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

func NewTaskConflictError(lockKey string) error {
	return fmt.Errorf("%w: %s", ErrTaskConflict, lockKey)
}
