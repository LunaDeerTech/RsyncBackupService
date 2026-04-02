package executor

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

func TestBuildRollingPlanForRemoteToRemote(t *testing.T) {
	instance := model.BackupInstance{
		ID:         42,
		Name:       "db-prod",
		SourceType: "remote",
	}
	target := model.StorageTarget{
		ID:       99,
		Type:     "rolling_ssh",
		BasePath: "/srv/backups",
	}

	plan := BuildRollingPlan(instance, target)
	if !plan.RequiresRelay {
		t.Fatal("expected relay mode for remote to remote rolling backup")
	}
	if !strings.HasPrefix(plan.SnapshotPath, filepath.Join(target.BasePath, SnapshotRootDir(instance))+string(filepath.Separator)) {
		t.Fatalf("expected snapshot path under instance directory, got %q", plan.SnapshotPath)
	}
	if !strings.Contains(plan.RelayCacheDir, filepath.Join("relay_cache", "42")) {
		t.Fatalf("expected relay cache path to include instance cache root, got %q", plan.RelayCacheDir)
	}
	if !strings.Contains(plan.RelayCacheDir, filepath.Join("42", "99")) {
		t.Fatalf("expected relay cache path to isolate target id, got %q", plan.RelayCacheDir)
	}
	if !strings.Contains(plan.LinkDest, filepath.Join(target.BasePath, SnapshotRootDir(instance))) {
		t.Fatalf("expected link-dest to stay under instance directory, got %q", plan.LinkDest)
	}
}

func TestMapRsyncExitCodeProducesReadableError(t *testing.T) {
	err := MapRsyncExitCode(23)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "partial transfer") {
		t.Fatalf("expected readable rsync error, got %v", err)
	}
}

func TestWithExecutionTimeoutWrapsContext(t *testing.T) {
	ctx, cancel := WithExecutionTimeout(context.Background(), 1)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected timeout context to have a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 2*time.Second {
		t.Fatalf("expected deadline about one second out, got %s", remaining)
	}

	uncapped, stop := WithExecutionTimeout(context.Background(), 0)
	defer stop()
	if _, ok := uncapped.Deadline(); ok {
		t.Fatal("expected zero max execution seconds to use cancel-only context")
	}
}

func TestBuildRollingPlanUsesSubSecondTimestampPrecision(t *testing.T) {
	originalNow := buildRollingPlanNow
	defer func() {
		buildRollingPlanNow = originalNow
	}()

	timestamps := []time.Time{
		time.Date(2026, 4, 2, 12, 0, 0, 123, time.UTC),
		time.Date(2026, 4, 2, 12, 0, 0, 456, time.UTC),
	}
	currentIndex := 0
	buildRollingPlanNow = func() time.Time {
		current := timestamps[currentIndex]
		currentIndex++
		return current
	}

	instance := model.BackupInstance{Name: "db-prod", SourceType: "local"}
	target := model.StorageTarget{Type: "rolling_local", BasePath: "/srv/backups"}

	firstPlan := BuildRollingPlan(instance, target)
	secondPlan := BuildRollingPlan(instance, target)
	if firstPlan.SnapshotPath == secondPlan.SnapshotPath {
		t.Fatalf("expected distinct snapshot paths for same-second runs, got %q", firstPlan.SnapshotPath)
	}
}

func TestSnapshotRootDirStableAcrossRename(t *testing.T) {
	beforeRename := SnapshotRootDir(model.BackupInstance{ID: 42, Name: "db-prod"})
	afterRename := SnapshotRootDir(model.BackupInstance{ID: 42, Name: "db-archive"})
	if beforeRename != afterRename {
		t.Fatalf("expected snapshot root to stay stable across rename, got %q and %q", beforeRename, afterRename)
	}
}

func TestExecuteRollingBuildsRelayCommandsAndReportsProgress(t *testing.T) {
	runner := &runnerSpy{lines: []string{"1,024  50%  1.00MB/s  0:00:01"}}
	request := RollingExecutionRequest{
		Instance: model.BackupInstance{
			ID:         7,
			SourceType: "remote",
			SourceHost: "source.example.com",
			SourcePort: 22,
			SourceUser: "root",
			SourcePath: "/srv/source",
		},
		Target: model.StorageTarget{
			Type: "rolling_ssh",
			Host: "backup.example.com",
			Port: 2222,
			User: "backup",
		},
		SourceSSHKeyPath: "/keys/source_id_rsa",
		TargetSSHKeyPath: "/keys/target_id_rsa",
		SnapshotPath:     "/srv/backup/db-prod/20260402T120000Z",
		TargetLinkDest:   "/srv/backup/db-prod/20260401T120000Z",
		RelayCacheDir:    "/var/lib/rbs/relay_cache/7/20260402T120000Z",
		RelayLinkDest:    "/var/lib/rbs/relay_cache/7/20260401T120000Z",
	}

	var snapshots []ProgressSnapshot
	if err := ExecuteRolling(context.Background(), runner, request, func(snapshot ProgressSnapshot) {
		snapshots = append(snapshots, snapshot)
	}); err != nil {
		t.Fatalf("execute rolling backup: %v", err)
	}

	if len(runner.specs) != 2 {
		t.Fatalf("expected two rsync command specs for relay mode, got %d", len(runner.specs))
	}
	if !slices.Contains(runner.specs[0].Args, "--link-dest=/var/lib/rbs/relay_cache/7/20260401T120000Z") {
		t.Fatalf("expected relay pull command to use relay link-dest, args=%v", runner.specs[0].Args)
	}
	if !slices.Contains(runner.specs[1].Args, "--link-dest=/srv/backup/db-prod/20260401T120000Z") {
		t.Fatalf("expected relay push command to use target link-dest, args=%v", runner.specs[1].Args)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected progress callback for both relay phases, got %d", len(snapshots))
	}
	if snapshots[0].EstimatedRemaining <= 0 {
		t.Fatalf("expected progress callback to include ETA, got %+v", snapshots[0])
	}
	if snapshots[0].AverageBytesPerSecond <= 0 {
		t.Fatalf("expected progress callback to include average speed, got %+v", snapshots[0])
	}
}

func TestExecuteRollingAggregatesRelayProgressAcrossPhases(t *testing.T) {
	runner := &runnerSpy{phaseLines: [][]string{{"2,048  100%  1.00MB/s  0:00:01"}, {"1,024  50%  1.00MB/s  0:00:01"}}}
	request := RollingExecutionRequest{
		Instance: model.BackupInstance{
			ID:         7,
			SourceType: "remote",
			SourceHost: "source.example.com",
			SourcePort: 22,
			SourceUser: "root",
			SourcePath: "/srv/source",
		},
		Target: model.StorageTarget{
			Type: "rolling_ssh",
			Host: "backup.example.com",
			Port: 2222,
			User: "backup",
		},
		SourceSSHKeyPath: "/keys/source_id_rsa",
		TargetSSHKeyPath: "/keys/target_id_rsa",
		SnapshotPath:     "/srv/backup/db-prod/20260402T120000.000000001Z",
		TargetLinkDest:   "/srv/backup/db-prod/20260401T120000.000000001Z",
		RelayCacheDir:    "/var/lib/rbs/relay_cache/7/20260402T120000.000000001Z",
		RelayLinkDest:    "/var/lib/rbs/relay_cache/7/20260401T120000.000000001Z",
	}

	var snapshots []ProgressSnapshot
	if err := ExecuteRolling(context.Background(), runner, request, func(snapshot ProgressSnapshot) {
		snapshots = append(snapshots, snapshot)
	}); err != nil {
		t.Fatalf("execute rolling backup: %v", err)
	}

	if len(snapshots) != 2 {
		t.Fatalf("expected two aggregated relay progress snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Percentage != 50 {
		t.Fatalf("expected first relay phase to report 50%% overall progress, got %d", snapshots[0].Percentage)
	}
	if snapshots[1].Percentage != 75 {
		t.Fatalf("expected second relay phase midpoint to report 75%% overall progress, got %d", snapshots[1].Percentage)
	}
	if snapshots[0].BytesTransferred != 2048 {
		t.Fatalf("expected first relay phase to report one full phase of transferred bytes, got %d", snapshots[0].BytesTransferred)
	}
	if snapshots[1].BytesTransferred != 3072 {
		t.Fatalf("expected second relay midpoint to report cumulative transferred bytes, got %d", snapshots[1].BytesTransferred)
	}
	if snapshots[0].EstimatedRemaining <= snapshots[1].EstimatedRemaining {
		t.Fatalf("expected remaining ETA to shrink across phases, got first=%s second=%s", snapshots[0].EstimatedRemaining, snapshots[1].EstimatedRemaining)
	}
}

type runnerSpy struct {
	specs []CommandSpec
	lines []string
	phaseLines [][]string
	err   error
	runs  int
}

func (r *runnerSpy) Run(_ context.Context, spec CommandSpec, onStdout func(string)) error {
	r.specs = append(r.specs, spec)
	lines := r.lines
	if len(r.phaseLines) > 0 && r.runs < len(r.phaseLines) {
		lines = r.phaseLines[r.runs]
	}
	r.runs++
	for _, line := range lines {
		onStdout(line)
	}
	return r.err
}