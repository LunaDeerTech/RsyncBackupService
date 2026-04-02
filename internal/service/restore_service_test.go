package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"gorm.io/gorm"
)

func TestRestoreRequiresVerifyToken(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)

	_, err := fixture.service.Start(context.Background(), RestoreRequest{})
	if err == nil {
		t.Fatal("expected verify token validation error")
	}
	if err != ErrVerifyTokenRequired {
		t.Fatalf("expected ErrVerifyTokenRequired, got %v", err)
	}
}

func TestRestoreRejectsInvalidVerifyToken(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)

	_, err := fixture.service.Start(context.Background(), RestoreRequest{
		InstanceID:        fixture.instance.ID,
		BackupRecordID:    fixture.backupRecord.ID,
		RestoreTargetPath: filepath.Join(t.TempDir(), "restore-target"),
		Overwrite:         false,
		VerifyToken:       "invalid-token",
		TriggeredBy:       fixture.admin.ID,
	})
	if err != ErrVerifyTokenInvalid {
		t.Fatalf("expected ErrVerifyTokenInvalid, got %v", err)
	}
}

func TestRestoreStartPersistsRecord(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)
	restoreTargetPath := filepath.Join(t.TempDir(), "restore-target")

	record, err := fixture.service.Start(context.Background(), RestoreRequest{
		InstanceID:        fixture.instance.ID,
		BackupRecordID:    fixture.backupRecord.ID,
		RestoreTargetPath: restoreTargetPath,
		Overwrite:         false,
		VerifyToken:       fixture.issueVerifyToken(t),
		TriggeredBy:       fixture.admin.ID,
	})
	if err != nil {
		t.Fatalf("start restore: %v", err)
	}
	if record.ID == 0 {
		t.Fatal("expected persisted restore record id")
	}
	if record.Status != RestoreStatusSuccess {
		t.Fatalf("expected restore status %q, got %q", RestoreStatusSuccess, record.Status)
	}

	var stored model.RestoreRecord
	if err := fixture.db.First(&stored, record.ID).Error; err != nil {
		t.Fatalf("load restore record: %v", err)
	}
	if stored.BackupRecordID != fixture.backupRecord.ID {
		t.Fatalf("expected backup record id %d, got %d", fixture.backupRecord.ID, stored.BackupRecordID)
	}
	if stored.RestoreTargetPath != restoreTargetPath {
		t.Fatalf("expected restore target path %q, got %q", restoreTargetPath, stored.RestoreTargetPath)
	}
	if stored.TriggeredBy != fixture.admin.ID {
		t.Fatalf("expected triggered by %d, got %d", fixture.admin.ID, stored.TriggeredBy)
	}
	if stored.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}
}

func TestEnsureLocalRestoreTargetRejectsSymlinkTarget(t *testing.T) {
	targetRoot := t.TempDir()
	symlinkPath := filepath.Join(t.TempDir(), "restore-link")
	if err := os.Symlink(targetRoot, symlinkPath); err != nil {
		t.Fatalf("create restore symlink: %v", err)
	}

	err := ensureLocalRestoreTarget(symlinkPath, false)
	if err != ErrInvalidRestoreTargetPath {
		t.Fatalf("expected ErrInvalidRestoreTargetPath, got %v", err)
	}
}

func TestEnsureLocalRestoreTargetRejectsSymlinkParent(t *testing.T) {
	outsideRoot := t.TempDir()
	symlinkParent := filepath.Join(t.TempDir(), "restore-parent")
	if err := os.Symlink(outsideRoot, symlinkParent); err != nil {
		t.Fatalf("create restore parent symlink: %v", err)
	}

	err := ensureLocalRestoreTarget(filepath.Join(symlinkParent, "nested"), false)
	if err != ErrInvalidRestoreTargetPath {
		t.Fatalf("expected ErrInvalidRestoreTargetPath, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outsideRoot, "nested")); !os.IsNotExist(err) {
		t.Fatalf("expected no directory creation through symlinked parent, stat err=%v", err)
	}
}

func TestRestoreStartCompletesFailedRecordAfterExecutionContextCancelled(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)
	restoreTargetPath := filepath.Join(t.TempDir(), "restore-target")
	ctx, cancel := context.WithCancel(context.Background())
	fixture.service.runner = restoreRunnerFunc(func(context.Context, executorpkg.CommandSpec, func(string)) error {
		cancel()
		return context.Canceled
	})

	_, err := fixture.service.Start(ctx, RestoreRequest{
		InstanceID:        fixture.instance.ID,
		BackupRecordID:    fixture.backupRecord.ID,
		RestoreTargetPath: restoreTargetPath,
		Overwrite:         false,
		VerifyToken:       fixture.issueVerifyToken(t),
		TriggeredBy:       fixture.admin.ID,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}

	var stored model.RestoreRecord
	if err := fixture.db.First(&stored).Error; err != nil {
		t.Fatalf("load restore record: %v", err)
	}
	if stored.Status != RestoreStatusFailed {
		t.Fatalf("expected restore status %q, got %q", RestoreStatusFailed, stored.Status)
	}
	if stored.FinishedAt == nil {
		t.Fatal("expected failed restore record to set finished_at")
	}
	if !strings.Contains(stored.ErrorMessage, context.Canceled.Error()) {
		t.Fatalf("expected restore error message to include cancellation, got %q", stored.ErrorMessage)
	}
}

func TestRestoreStartHandlesSingleSplitArchivePart(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)
	restoreTargetPath := filepath.Join(t.TempDir(), "restore-target")
	archivePartPath := executorpkg.SplitArchivePartPath(fixture.backupRecord.SnapshotPath, 0)
	if err := os.Remove(fixture.backupRecord.SnapshotPath); err != nil {
		t.Fatalf("remove unsplit archive: %v", err)
	}
	if err := os.WriteFile(archivePartPath, []byte("split-archive"), 0o644); err != nil {
		t.Fatalf("write split archive part: %v", err)
	}
	runner := &restoreRunnerSpy{}
	fixture.service.runner = runner

	record, err := fixture.service.Start(context.Background(), RestoreRequest{
		InstanceID:        fixture.instance.ID,
		BackupRecordID:    fixture.backupRecord.ID,
		RestoreTargetPath: restoreTargetPath,
		Overwrite:         false,
		VerifyToken:       fixture.issueVerifyToken(t),
		TriggeredBy:       fixture.admin.ID,
	})
	if err != nil {
		t.Fatalf("start restore: %v", err)
	}
	if record.Status != RestoreStatusSuccess {
		t.Fatalf("expected restore status %q, got %q", RestoreStatusSuccess, record.Status)
	}

	foundSplitExtract := false
	for _, spec := range runner.specs {
		if spec.Name == "sh" && len(spec.Args) >= 2 && strings.Contains(spec.Args[1], "cat ") {
			foundSplitExtract = true
			break
		}
	}
	if !foundSplitExtract {
		t.Fatalf("expected split archive extract command, got %#v", runner.specs)
	}
}

type restoreServiceTestFixture struct {
	db           *gorm.DB
	service      *RestoreService
	auth         *AuthService
	admin        model.User
	instance     model.BackupInstance
	storageTarget model.StorageTarget
	backupRecord model.BackupRecord
}

func newRestoreServiceTestFixture(t *testing.T) restoreServiceTestFixture {
	t.Helper()

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := repository.MigrateAndSeed(db, config.Config{AdminUser: "admin", AdminPassword: "secret"}); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}

	instance := model.BackupInstance{
		Name:            "db-prod",
		SourceType:      SourceTypeLocal,
		SourcePath:      filepath.Join(t.TempDir(), "source"),
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	target := model.StorageTarget{
		Name:     "cold-target",
		Type:     StorageTargetTypeColdLocal,
		BasePath: filepath.Join(t.TempDir(), "cold-target"),
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}

	archivePath := filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance), "db-prod_20260402T120000Z.tar.gz")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("not-a-real-archive"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	finishedAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	backupRecord := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        BackupTypeCold,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: storageTargetLocationKey(target),
		SnapshotPath:      archivePath,
		VolumeCount:       1,
		StartedAt:         finishedAt.Add(-time.Minute),
		FinishedAt:        &finishedAt,
	}
	if err := db.Create(&backupRecord).Error; err != nil {
		t.Fatalf("create backup record: %v", err)
	}

	authService := NewAuthService(db, "restore-service-test-secret")
	runner := &restoreRunnerSpy{}
	restoreService := NewRestoreService(db, config.Config{DataDir: t.TempDir()}, runner, authService)

	return restoreServiceTestFixture{
		db:            db,
		service:       restoreService,
		auth:          authService,
		admin:         admin,
		instance:      instance,
		storageTarget: target,
		backupRecord:  backupRecord,
	}
}

func (f restoreServiceTestFixture) issueVerifyToken(t *testing.T) string {
	t.Helper()

	verifyToken, err := f.auth.VerifyPassword(context.Background(), f.admin.ID, "secret")
	if err != nil {
		t.Fatalf("issue verify token: %v", err)
	}

	return verifyToken
}

type restoreRunnerSpy struct {
	specs []executorpkg.CommandSpec
	err   error
}

func (r *restoreRunnerSpy) Run(_ context.Context, spec executorpkg.CommandSpec, _ func(string)) error {
	r.specs = append(r.specs, spec)
	return r.err
}

type restoreRunnerFunc func(context.Context, executorpkg.CommandSpec, func(string)) error

func (f restoreRunnerFunc) Run(ctx context.Context, spec executorpkg.CommandSpec, onStdout func(string)) error {
	return f(ctx, spec, onStdout)
}