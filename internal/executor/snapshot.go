package executor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

type RollingPlan struct {
	RequiresRelay bool
	SnapshotPath  string
	RelayCacheDir string
	LinkDest      string
}

func SnapshotRootName(name string) string {
	return sanitizePathSegment(name)
}

func SnapshotRootDir(instance model.BackupInstance) string {
	if instance.ID == 0 {
		return SnapshotRootName(instance.Name)
	}

	return fmt.Sprintf("instance-%d", instance.ID)
}

var buildRollingPlanNow = func() time.Time {
	return time.Now().UTC()
}

func BuildRollingPlan(instance model.BackupInstance, target model.StorageTarget) RollingPlan {
	instanceDir := SnapshotRootDir(instance)
	timestamp := buildRollingPlanNow().UTC().Format("20060102T150405.000000000Z")

	plan := RollingPlan{
		SnapshotPath: filepath.Clean(filepath.Join(target.BasePath, instanceDir, timestamp)),
		LinkDest:     filepath.Clean(filepath.Join(target.BasePath, instanceDir, "latest")),
	}

	if isRemoteSource(instance) && isRemoteTarget(target) {
		plan.RequiresRelay = true
		plan.RelayCacheDir = filepath.Join("relay_cache", strconv.FormatUint(uint64(instance.ID), 10), strconv.FormatUint(uint64(target.ID), 10), timestamp)
	}

	return plan
}

func sanitizePathSegment(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return "snapshot"
	}

	var builder strings.Builder
	builder.Grow(len(trimmedValue))
	for _, currentRune := range trimmedValue {
		switch {
		case unicode.IsLetter(currentRune), unicode.IsDigit(currentRune):
			builder.WriteRune(currentRune)
		case currentRune == '-', currentRune == '_', currentRune == '.':
			builder.WriteRune(currentRune)
		case unicode.IsSpace(currentRune):
			builder.WriteByte('-')
		}
	}

	result := strings.Trim(builder.String(), "-._")
	if result == "" {
		return "snapshot"
	}

	return result
}

func isRemoteSource(instance model.BackupInstance) bool {
	return strings.EqualFold(strings.TrimSpace(instance.SourceType), "remote")
}

func isRemoteTarget(target model.StorageTarget) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(target.Type)), "_ssh")
}