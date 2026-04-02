package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
)

func TestRetentionServiceCleanupDeletesUnionOfCountAndAge(t *testing.T) {
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
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	target := model.StorageTarget{
		Name:     "primary",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: t.TempDir(),
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	strategy := model.Strategy{
		InstanceID:      instance.ID,
		Name:            "daily",
		BackupType:      BackupTypeRolling,
		IntervalSeconds: 3600,
		RetentionDays:   5,
		RetentionCount:  2,
		Enabled:         true,
	}
	if err := db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	instanceDir := filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance))
	paths := []string{
		filepath.Join(instanceDir, "20260323T120000Z"),
		filepath.Join(instanceDir, "20260329T120000Z"),
		filepath.Join(instanceDir, "20260330T120000Z"),
		filepath.Join(instanceDir, "20260331T120000Z"),
	}
	finishedTimes := []time.Time{
		now.Add(-10 * 24 * time.Hour),
		now.Add(-4 * 24 * time.Hour),
		now.Add(-3 * 24 * time.Hour),
		now.Add(-2 * 24 * time.Hour),
	}
	strategyID := strategy.ID
	for index, snapshotPath := range paths {
		createSnapshotDir(t, snapshotPath, finishedTimes[index])
		record := model.BackupRecord{
			InstanceID:        instance.ID,
			StorageTargetID:   target.ID,
			StrategyID:        &strategyID,
			BackupType:        BackupTypeRolling,
			Status:            model.BackupStatusSuccess,
			TargetLocationKey: storageTargetLocationKey(target),
			SnapshotPath:      snapshotPath,
			StartedAt:         finishedTimes[index].Add(-time.Minute),
			FinishedAt:        &finishedTimes[index],
		}
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("create backup record: %v", err)
		}
	}

	retentionService := NewRetentionService(db)
	retentionService.clock = func() time.Time {
		return now
	}

	if err := retentionService.Cleanup(context.Background(), strategy, target); err != nil {
		t.Fatalf("cleanup retention: %v", err)
	}

	assertPathMissing(t, paths[0])
	assertPathMissing(t, paths[1])
	assertPathExists(t, paths[2])
	assertPathExists(t, paths[3])
}

func TestExecutorServiceCheckTargetSpaceWarnsButDoesNotPanic(t *testing.T) {
	service := &ExecutorService{}
	backend := fakeStorageBackend{available: 512}

	err := service.CheckTargetSpace(context.Background(), backend, ".", 1024)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "insufficient") {
		t.Fatalf("expected insufficient space warning, got %v", err)
	}

	if err := service.CheckTargetSpace(context.Background(), backend, ".", 0); err != nil {
		t.Fatalf("expected zero estimated size to skip warning, got %v", err)
	}
}

func TestExecutorServiceEnsureLocalExecutionPathsUsesOnlyRelayCacheForSSHTarget(t *testing.T) {
	dataDir := t.TempDir()
	remoteBaseDir := filepath.Join(t.TempDir(), "remote-target-root")
	service := &ExecutorService{config: config.Config{DataDir: dataDir}}
	target := model.StorageTarget{Type: StorageTargetTypeRollingSSH}
	plan := executorpkg.RollingPlan{
		SnapshotPath:  filepath.Join(remoteBaseDir, "instance-a", "20260402T120000Z"),
		RelayCacheDir: filepath.Join("relay_cache", "42", "20260402T120000Z"),
	}

	if err := service.ensureLocalExecutionPaths(target, plan); err != nil {
		t.Fatalf("ensure local execution paths: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(plan.SnapshotPath)); !os.IsNotExist(err) {
		t.Fatalf("expected remote target parent not to be created locally, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Join(dataDir, plan.RelayCacheDir))); err != nil {
		t.Fatalf("expected relay cache parent to be created locally: %v", err)
	}
}

func TestExecutorServiceRunStrategyMarksRecordFailedWhenCleanupFails(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)

	strategy := model.Strategy{
		InstanceID:      fixture.instance.ID,
		Name:            "hourly",
		BackupType:      BackupTypeRolling,
		IntervalSeconds: 3600,
		Enabled:         true,
		StorageTargets:  []model.StorageTarget{target},
	}
	if err := fixture.db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	service := NewExecutorService(fixture.db, config.Config{DataDir: t.TempDir()}, successfulRunner{}, executorpkg.NewTaskManager())
	service.retentionService = retentionCleanerFunc(func(context.Context, model.Strategy, model.StorageTarget) error {
		return errors.New("cleanup failed")
	})

	err := service.RunStrategy(context.Background(), strategy)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected cleanup failure from run strategy, got %v", err)
	}

	var records []model.BackupRecord
	if err := fixture.db.Order("id ASC").Find(&records).Error; err != nil {
		t.Fatalf("list backup records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one backup record, got %d", len(records))
	}
	if records[0].Status != model.BackupStatusFailed {
		t.Fatalf("expected backup record status %q, got %q", model.BackupStatusFailed, records[0].Status)
	}
	if !strings.Contains(records[0].ErrorMessage, "cleanup failed") {
		t.Fatalf("expected backup record to store cleanup error, got %q", records[0].ErrorMessage)
	}
}

func TestExecutorServiceRunStrategyCreatesFailedRecordWhenLocalPathPreparationFails(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	blockingPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockingPath, []byte("block"), 0o644); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	target := model.StorageTarget{
		Name:     "broken-target",
		Type:     StorageTargetTypeRollingLocal,
		BasePath: blockingPath,
	}
	if err := fixture.db.Create(&target).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}

	strategy := model.Strategy{
		InstanceID:      fixture.instance.ID,
		Name:            "hourly-broken",
		BackupType:      BackupTypeRolling,
		IntervalSeconds: 3600,
		Enabled:         true,
		StorageTargets:  []model.StorageTarget{target},
	}
	if err := fixture.db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	service := NewExecutorService(fixture.db, config.Config{DataDir: t.TempDir()}, successfulRunner{}, executorpkg.NewTaskManager())
	err := service.RunStrategy(context.Background(), strategy)
	if err == nil {
		t.Fatal("expected local path preparation failure")
	}

	var records []model.BackupRecord
	if err := fixture.db.Order("id ASC").Find(&records).Error; err != nil {
		t.Fatalf("list backup records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one failed backup record, got %d", len(records))
	}
	if records[0].Status != model.BackupStatusFailed {
		t.Fatalf("expected failed backup record status, got %q", records[0].Status)
	}
}

func TestFindLatestRelayCacheUsesOnlyCompletedMarkers(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "relay_cache", "42", "99")
	completedDir := filepath.Join(cacheRoot, "20260402T120000.000000001Z")
	incompleteDir := filepath.Join(cacheRoot, "20260403T120000.000000001Z")
	currentDir := filepath.Join(cacheRoot, "20260404T120000.000000001Z")
	for _, directory := range []string{completedDir, incompleteDir, currentDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("create relay cache dir %q: %v", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(completedDir, relayCacheSuccessMarkerName), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write relay completion marker: %v", err)
	}

	service := &ExecutorService{}
	latestRelayCache, err := service.findLatestRelayCache(currentDir)
	if err != nil {
		t.Fatalf("find latest relay cache: %v", err)
	}
	if latestRelayCache != completedDir {
		t.Fatalf("expected latest completed relay cache %q, got %q", completedDir, latestRelayCache)
	}
}

func TestExecutorServiceIgnoresSnapshotsFromStaleTargetLocations(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	service := NewExecutorService(fixture.db, config.Config{}, nil, executorpkg.NewTaskManager())
	finishedAt := time.Now().UTC()

	record := model.BackupRecord{
		InstanceID:        fixture.instance.ID,
		StorageTargetID:   target.ID,
		BackupType:        BackupTypeRolling,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: storageTargetLocationKey(target),
		SnapshotPath:      filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260402T120000.000000001Z"),
		StartedAt:         finishedAt.Add(-time.Minute),
		FinishedAt:        &finishedAt,
	}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create backup record: %v", err)
	}

	target.BasePath = filepath.Join(t.TempDir(), "moved-target")
	linkDest, err := service.findLatestSuccessfulSnapshot(fixture.instance.ID, target)
	if err != nil {
		t.Fatalf("find latest successful snapshot: %v", err)
	}
	if linkDest != "" {
		t.Fatalf("expected moved target to ignore stale snapshot history, got %q", linkDest)
	}
}

func TestExecutorServiceUsesLegacySnapshotsWhenTargetLocationIsUnchanged(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	service := NewExecutorService(fixture.db, config.Config{}, nil, executorpkg.NewTaskManager())
	finishedAt := time.Now().UTC()
	expectedPath := filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260402T120000.000000001Z")

	record := model.BackupRecord{
		InstanceID:      fixture.instance.ID,
		StorageTargetID: target.ID,
		BackupType:      BackupTypeRolling,
		Status:          model.BackupStatusSuccess,
		SnapshotPath:    expectedPath,
		StartedAt:       finishedAt.Add(-time.Minute),
		FinishedAt:      &finishedAt,
	}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create legacy backup record: %v", err)
	}

	linkDest, err := service.findLatestSuccessfulSnapshot(fixture.instance.ID, target)
	if err != nil {
		t.Fatalf("find latest successful snapshot: %v", err)
	}
	if linkDest != expectedPath {
		t.Fatalf("expected unchanged target to reuse legacy snapshot %q, got %q", expectedPath, linkDest)
	}
}

func TestExecutorServiceReusesSnapshotsAcrossLegacyDuplicateTargetsWithSameLocation(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	firstTarget := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target-a", StorageTargetTypeRollingLocal)
	secondTarget := model.StorageTarget{Name: "rolling-target-b", Type: StorageTargetTypeRollingLocal, BasePath: firstTarget.BasePath}
	if err := fixture.db.Create(&secondTarget).Error; err != nil {
		t.Fatalf("create second legacy target: %v", err)
	}
	service := NewExecutorService(fixture.db, config.Config{}, nil, executorpkg.NewTaskManager())
	finishedAt := time.Now().UTC()
	expectedPath := filepath.Join(firstTarget.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260402T120000.000000001Z")

	record := model.BackupRecord{
		InstanceID:        fixture.instance.ID,
		StorageTargetID:   firstTarget.ID,
		BackupType:        BackupTypeRolling,
		Status:            model.BackupStatusSuccess,
		TargetLocationKey: storageTargetLocationKey(firstTarget),
		SnapshotPath:      expectedPath,
		StartedAt:         finishedAt.Add(-time.Minute),
		FinishedAt:        &finishedAt,
	}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create backup record: %v", err)
	}

	linkDest, err := service.findLatestSuccessfulSnapshot(fixture.instance.ID, secondTarget)
	if err != nil {
		t.Fatalf("find latest successful snapshot: %v", err)
	}
	if linkDest != expectedPath {
		t.Fatalf("expected duplicate target rows with same location to reuse snapshot %q, got %q", expectedPath, linkDest)
	}
}

func TestExecutorServiceDoesNotReuseLegacySnapshotsAcrossSSHTargets(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	service := NewExecutorService(fixture.db, config.Config{}, nil, executorpkg.NewTaskManager())
	sshKey := model.SSHKey{Name: "ssh-key", PrivateKeyPath: "/tmp/test-key", Fingerprint: "test"}
	if err := fixture.db.Create(&sshKey).Error; err != nil {
		t.Fatalf("create ssh key: %v", err)
	}
	firstTarget := model.StorageTarget{
		Name:     "ssh-a",
		Type:     StorageTargetTypeRollingSSH,
		Host:     "backup-a.example.com",
		Port:     22,
		User:     "backup",
		SSHKeyID: &sshKey.ID,
		BasePath: "/srv/backups",
	}
	secondTarget := model.StorageTarget{
		Name:     "ssh-b",
		Type:     StorageTargetTypeRollingSSH,
		Host:     "backup-b.example.com",
		Port:     22,
		User:     "backup",
		SSHKeyID: &sshKey.ID,
		BasePath: "/srv/backups",
	}
	if err := fixture.db.Create(&firstTarget).Error; err != nil {
		t.Fatalf("create first ssh target: %v", err)
	}
	if err := fixture.db.Create(&secondTarget).Error; err != nil {
		t.Fatalf("create second ssh target: %v", err)
	}
	finishedAt := time.Now().UTC()
	record := model.BackupRecord{
		InstanceID:      fixture.instance.ID,
		StorageTargetID: firstTarget.ID,
		BackupType:      BackupTypeRolling,
		Status:          model.BackupStatusSuccess,
		SnapshotPath:    filepath.Join(firstTarget.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260402T120000.000000001Z"),
		StartedAt:       finishedAt.Add(-time.Minute),
		FinishedAt:      &finishedAt,
	}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create legacy ssh backup record: %v", err)
	}

	linkDest, err := service.findLatestSuccessfulSnapshot(fixture.instance.ID, secondTarget)
	if err != nil {
		t.Fatalf("find latest successful snapshot: %v", err)
	}
	if linkDest != "" {
		t.Fatalf("expected ssh legacy fallback to stay disabled, got %q", linkDest)
	}
}

func TestExecutorServiceProgressRecordBytesStayConsistentWithTotalSize(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	service := NewExecutorService(fixture.db, config.Config{}, nil, executorpkg.NewTaskManager())
	record := model.BackupRecord{
		InstanceID:      fixture.instance.ID,
		StorageTargetID: target.ID,
		BackupType:      BackupTypeRolling,
		Status:          model.BackupStatusRunning,
		StartedAt:       time.Now().UTC(),
	}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create backup record: %v", err)
	}

	snapshot := executorpkg.ProgressSnapshot{
		BytesTransferred:   3072,
		PhaseBytesTransferred: 1024,
		Percentage:         75,
		PhasePercentage:    50,
		EstimatedTotalSize: 2048,
	}
	if err := service.updateBackupRecordProgress(record.ID, snapshot); err != nil {
		t.Fatalf("update backup record progress: %v", err)
	}

	var persisted model.BackupRecord
	if err := fixture.db.First(&persisted, record.ID).Error; err != nil {
		t.Fatalf("reload backup record: %v", err)
	}
	if persisted.TotalSize != 2048 {
		t.Fatalf("expected persisted total size 2048, got %d", persisted.TotalSize)
	}
	if persisted.BytesTransferred != 1536 {
		t.Fatalf("expected persisted logical bytes transferred 1536, got %d", persisted.BytesTransferred)
	}
}

func TestRetentionServiceCleanupIgnoresFailedSnapshots(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	strategy := model.Strategy{Name: "rolling-retention", InstanceID: fixture.instance.ID, BackupType: BackupTypeRolling, IntervalSeconds: 3600, RetentionCount: 2, Enabled: true}
	if err := fixture.db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}
	service := NewRetentionService(fixture.db)
	now := time.Now().UTC()
	paths := []string{
		filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260401T010000.000000001Z"),
		filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260402T010000.000000001Z"),
		filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260403T010000.000000001Z"),
		filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(fixture.instance), "20260404T010000.000000001Z"),
	}
	for index, snapshotPath := range paths {
		createSnapshotDir(t, snapshotPath, now.Add(time.Duration(index)*time.Minute))
	}
	finishedAt1 := now.Add(-4 * time.Hour)
	finishedAt2 := now.Add(-3 * time.Hour)
	finishedAt3 := now.Add(-2 * time.Hour)
	strategyID := strategy.ID
	records := []model.BackupRecord{
		{InstanceID: fixture.instance.ID, StorageTargetID: target.ID, StrategyID: &strategyID, BackupType: BackupTypeRolling, Status: model.BackupStatusSuccess, TargetLocationKey: storageTargetLocationKey(target), SnapshotPath: paths[0], StartedAt: finishedAt1.Add(-time.Minute), FinishedAt: &finishedAt1},
		{InstanceID: fixture.instance.ID, StorageTargetID: target.ID, StrategyID: &strategyID, BackupType: BackupTypeRolling, Status: model.BackupStatusSuccess, TargetLocationKey: storageTargetLocationKey(target), SnapshotPath: paths[1], StartedAt: finishedAt2.Add(-time.Minute), FinishedAt: &finishedAt2},
		{InstanceID: fixture.instance.ID, StorageTargetID: target.ID, StrategyID: &strategyID, BackupType: BackupTypeRolling, Status: model.BackupStatusSuccess, TargetLocationKey: storageTargetLocationKey(target), SnapshotPath: paths[2], StartedAt: finishedAt3.Add(-time.Minute), FinishedAt: &finishedAt3},
		{InstanceID: fixture.instance.ID, StorageTargetID: target.ID, StrategyID: &strategyID, BackupType: BackupTypeRolling, Status: model.BackupStatusFailed, TargetLocationKey: storageTargetLocationKey(target), SnapshotPath: paths[3], StartedAt: now.Add(-time.Minute)},
	}
	for _, record := range records {
		if err := fixture.db.Create(&record).Error; err != nil {
			t.Fatalf("create backup record: %v", err)
		}
	}

	if err := service.Cleanup(context.Background(), strategy, target); err != nil {
		t.Fatalf("cleanup retention: %v", err)
	}

	assertPathMissing(t, paths[0])
	assertPathExists(t, paths[1])
	assertPathExists(t, paths[2])
	assertPathExists(t, paths[3])
}

func createSnapshotDir(t *testing.T, path string, modifiedAt time.Time) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create snapshot dir %q: %v", path, err)
	}
	if err := os.Chtimes(path, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("set times for %q: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path %q to be deleted, stat err=%v", path, err)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path %q to exist: %v", path, err)
	}
}

type fakeStorageBackend struct {
	available uint64
	err       error
}

type retentionCleanerFunc func(context.Context, model.Strategy, model.StorageTarget) error

func (f retentionCleanerFunc) Cleanup(ctx context.Context, strategy model.Strategy, target model.StorageTarget) error {
	return f(ctx, strategy, target)
}

type successfulRunner struct{}

func (successfulRunner) Run(_ context.Context, _ executorpkg.CommandSpec, onStdout func(string)) error {
	if onStdout != nil {
		onStdout("1,024  100%  1.00MB/s  0:00:01")
	}
	return nil
}

func (b fakeStorageBackend) Type() string {
	return "fake"
}

func (b fakeStorageBackend) Upload(context.Context, string, string) error {
	return nil
}

func (b fakeStorageBackend) Download(context.Context, string, string) error {
	return nil
}

func (b fakeStorageBackend) List(context.Context, string) ([]storage.StorageObject, error) {
	return nil, nil
}

func (b fakeStorageBackend) Delete(context.Context, string) error {
	return nil
}

func (b fakeStorageBackend) SpaceAvailable(context.Context, string) (uint64, error) {
	return b.available, b.err
}

func (b fakeStorageBackend) TestConnection(context.Context) error {
	return nil
}