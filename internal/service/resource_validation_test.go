package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

func TestCreateInstanceGrantsCreatorAdminPermission(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	instanceService := NewInstanceService(fixture.db)

	instance, err := instanceService.Create(context.Background(), authIdentityFromUser(fixture.admin), CreateInstanceRequest{
		Name:            "prod-instance",
		SourceType:      "local",
		SourcePath:      "/srv/data",
		ExcludePatterns: []string{"*.tmp"},
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	var permission model.InstancePermission
	if err := fixture.db.Where("user_id = ? AND instance_id = ?", fixture.admin.ID, instance.ID).First(&permission).Error; err != nil {
		t.Fatalf("load instance permission: %v", err)
	}
	if permission.Role != RoleAdmin {
		t.Fatalf("expected creator role %q, got %q", RoleAdmin, permission.Role)
	}
	if instance.CreatedBy != fixture.admin.ID {
		t.Fatalf("expected created_by %d, got %d", fixture.admin.ID, instance.CreatedBy)
	}
}

func TestCreateStrategyRejectsMixedCronAndInterval(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "rolling-target", "rolling_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "nightly",
		BackupType:       "rolling",
		CronExpr:         ptrString("0 0 * * *"),
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "cron_expr and interval_seconds") {
		t.Fatalf("expected mixed schedule validation error, got %v", err)
	}
}

func TestCreateStrategyRejectsNegativeRetentionValues(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "cold-target", "cold_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "cold-backup",
		BackupType:       "cold",
		IntervalSeconds:  7200,
		RetentionDays:    -1,
		RetentionCount:   1,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "retention") {
		t.Fatalf("expected retention validation error, got %v", err)
	}
}

func TestCreateStrategyRejectsNegativeRetentionCount(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "rolling-target", "rolling_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-backup",
		BackupType:       "rolling",
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   -1,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "retention") {
		t.Fatalf("expected retention_count validation error, got %v", err)
	}
}

func TestCreateStrategyRejectsInvalidCronExpression(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "rolling-target", "rolling_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "broken-cron",
		BackupType:       "rolling",
		CronExpr:         ptrString("not-a-cron"),
		RetentionDays:    7,
		RetentionCount:   1,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "cron_expr") {
		t.Fatalf("expected cron validation error, got %v", err)
	}
}

func TestCreateStrategyRejectsIncompatibleStorageTargetType(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "cold-target", "cold_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-backup",
		BackupType:       "rolling",
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   1,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible storage target error, got %v", err)
	}
}

func TestCreateStrategyRejectsRollingStorageTargetConflictWithinInstance(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	target := createResourceValidationStorageTarget(t, fixture.db, "rolling-target", "rolling_local")

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-primary",
		BackupType:       "rolling",
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   1,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("seed first rolling strategy: %v", err)
	}

	_, err = strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-secondary",
		BackupType:       "rolling",
		IntervalSeconds:  7200,
		RetentionDays:    14,
		RetentionCount:   2,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "storage targets cannot be shared") {
		t.Fatalf("expected rolling storage target conflict error, got %v", err)
	}
}

func TestCreateStrategyRejectsLegacyDuplicateRollingLocationWithinInstance(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	sharedBasePath := filepath.Join(t.TempDir(), "shared-legacy-root")
	firstTarget := model.StorageTarget{Name: "legacy-a", Type: "rolling_local", BasePath: sharedBasePath}
	secondTarget := model.StorageTarget{Name: "legacy-b", Type: "rolling_local", BasePath: sharedBasePath}
	if err := fixture.db.Create(&firstTarget).Error; err != nil {
		t.Fatalf("create first legacy target: %v", err)
	}
	if err := fixture.db.Create(&secondTarget).Error; err != nil {
		t.Fatalf("create second legacy target: %v", err)
	}

	actor := authIdentityFromUser(fixture.admin)
	_, err := strategyService.Create(context.Background(), actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-a",
		BackupType:       "rolling",
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{firstTarget.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("seed first strategy: %v", err)
	}

	_, err = strategyService.Create(context.Background(), actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-b",
		BackupType:       "rolling",
		IntervalSeconds:  7200,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{secondTarget.ID},
		Enabled:          true,
	})
	if !errors.Is(err, ErrRollingTargetConflict) {
		t.Fatalf("expected legacy duplicate target location conflict, got %v", err)
	}
}

func TestCreateStrategyRejectsDuplicateLocationsWithinSingleRollingStrategy(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	strategyService := NewStrategyService(fixture.db)
	sharedBasePath := filepath.Join(t.TempDir(), "single-strategy-shared-root")
	firstTarget := model.StorageTarget{Name: "legacy-a", Type: "rolling_local", BasePath: sharedBasePath}
	secondTarget := model.StorageTarget{Name: "legacy-b", Type: "rolling_local", BasePath: sharedBasePath}
	if err := fixture.db.Create(&firstTarget).Error; err != nil {
		t.Fatalf("create first legacy target: %v", err)
	}
	if err := fixture.db.Create(&secondTarget).Error; err != nil {
		t.Fatalf("create second legacy target: %v", err)
	}

	_, err := strategyService.Create(context.Background(), authIdentityFromUser(fixture.admin), fixture.instance.ID, CreateStrategyRequest{
		Name:             "rolling-duplicate-targets",
		BackupType:       "rolling",
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{firstTarget.ID, secondTarget.ID},
		Enabled:          true,
	})
	if !errors.Is(err, ErrRollingTargetConflict) {
		t.Fatalf("expected duplicate locations within one rolling strategy to be rejected, got %v", err)
	}
}

func TestCreateStorageTargetRejectsDuplicateRollingLocation(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	storageTargetService := NewStorageTargetService(fixture.db)
	basePath := filepath.Join(t.TempDir(), "shared-rolling-target")

	_, err := storageTargetService.Create(context.Background(), CreateStorageTargetRequest{
		Name:     "primary",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: basePath,
	})
	if err != nil {
		t.Fatalf("create first storage target: %v", err)
	}

	_, err = storageTargetService.Create(context.Background(), CreateStorageTargetRequest{
		Name:     "secondary",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: basePath + string(filepath.Separator),
	})
	if !errors.Is(err, ErrDuplicateStorageTargetLocation) {
		t.Fatalf("expected duplicate storage target location error, got %v", err)
	}
	if !IsValidationError(err) {
		t.Fatalf("expected duplicate storage target location to be treated as validation error, got %v", err)
	}
}

func TestCreateStorageTargetRejectsLegacyNormalizedRollingLocation(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	storageTargetService := NewStorageTargetService(fixture.db)
	basePath := filepath.Join(t.TempDir(), "legacy-shared-rolling-target")
	legacyTarget := model.StorageTarget{Name: "legacy", Type: StorageTargetTypeRollingLocal, BasePath: basePath + string(filepath.Separator)}
	if err := fixture.db.Create(&legacyTarget).Error; err != nil {
		t.Fatalf("create legacy storage target: %v", err)
	}

	_, err := storageTargetService.Create(context.Background(), CreateStorageTargetRequest{
		Name:     "normalized",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: basePath,
	})
	if !errors.Is(err, ErrDuplicateStorageTargetLocation) {
		t.Fatalf("expected normalized duplicate rolling location error, got %v", err)
	}
}

func TestCreateStorageTargetRejectsLegacyNormalizedSSHLocation(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	storageTargetService := NewStorageTargetService(fixture.db)
	sshKey := model.SSHKey{Name: "legacy-key", PrivateKeyPath: "/tmp/legacy-key", Fingerprint: "legacy"}
	if err := fixture.db.Create(&sshKey).Error; err != nil {
		t.Fatalf("create ssh key: %v", err)
	}
	legacyTarget := model.StorageTarget{
		Name:     "legacy-ssh",
		Type:     StorageTargetTypeRollingSSH,
		Host:     "BACKUP.EXAMPLE.COM",
		Port:     0,
		User:     "backup",
		SSHKeyID: &sshKey.ID,
		BasePath: "/srv/backups/",
	}
	if err := fixture.db.Create(&legacyTarget).Error; err != nil {
		t.Fatalf("create legacy ssh target: %v", err)
	}

	_, err := storageTargetService.Create(context.Background(), CreateStorageTargetRequest{
		Name:     "normalized-ssh",
		Type:     StorageTargetTypeRollingSSH,
		Host:     "backup.example.com",
		Port:     22,
		User:     "backup",
		SSHKeyID: &sshKey.ID,
		BasePath: "/srv/backups",
	})
	if !errors.Is(err, ErrDuplicateStorageTargetLocation) {
		t.Fatalf("expected normalized duplicate ssh location error, got %v", err)
	}
}

func TestCreateStorageTargetRejectsBlankBasePath(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	storageTargetService := NewStorageTargetService(fixture.db)

	_, err := storageTargetService.Create(context.Background(), CreateStorageTargetRequest{
		Name:     "blank-base-path",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: "   ",
	})
	if !errors.Is(err, ErrBasePathRequired) {
		t.Fatalf("expected base path required error, got %v", err)
	}
}

func TestRegisterSSHKeyRejectsWorldReadableFile(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	sshKeyService := NewSSHKeyService(fixture.db)
	privateKeyPath := writeResourceValidationPrivateKey(t, 0o644)

	err := sshKeyService.Register(context.Background(), "prod", privateKeyPath)
	if err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("expected mode validation error, got %v", err)
	}
}

func TestSSHKeyTestConnectionRejectsPermissionsChangedKey(t *testing.T) {
	fixture := newResourceValidationFixture(t)
	sshKeyService := NewSSHKeyService(fixture.db)
	privateKeyPath := writeResourceValidationPrivateKey(t, 0o600)

	sshKey, err := sshKeyService.Create(context.Background(), CreateSSHKeyRequest{
		Name:           "prod",
		PrivateKeyPath: privateKeyPath,
	})
	if err != nil {
		t.Fatalf("create ssh key: %v", err)
	}

	if err := os.Chmod(privateKeyPath, 0o644); err != nil {
		t.Fatalf("chmod private key: %v", err)
	}

	err = sshKeyService.TestConnection(context.Background(), sshKey.ID, TestSSHKeyConnectionRequest{
		Host: "127.0.0.1",
		User: "root",
	})
	if err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("expected mode validation error during test connection, got %v", err)
	}
}

type resourceValidationFixture struct {
	db       *gorm.DB
	admin    model.User
	instance model.BackupInstance
}

func newResourceValidationFixture(t *testing.T) resourceValidationFixture {
	t.Helper()

	baseFixture := newAuthServiceTestFixture(t)
	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       baseFixture.admin.ID,
	}
	if err := baseFixture.db.Create(&instance).Error; err != nil {
		t.Fatalf("create backup instance: %v", err)
	}

	return resourceValidationFixture{
		db:       baseFixture.db,
		admin:    baseFixture.admin,
		instance: instance,
	}
}

func createResourceValidationStorageTarget(t *testing.T, db *gorm.DB, name, targetType string) model.StorageTarget {
	t.Helper()

	target := model.StorageTarget{
		Name:     name,
		Type:     targetType,
		BasePath: filepath.Join(t.TempDir(), name),
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	return target
}

func writeResourceValidationPrivateKey(t *testing.T, mode os.FileMode) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa private key: %v", err)
	}

	encodedKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	privateKeyPath := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(privateKeyPath, encodedKey, mode); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	return privateKeyPath
}

func ptrString(value string) *string {
	return &value
}
