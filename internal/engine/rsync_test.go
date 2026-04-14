package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
)

func TestBuildRsyncArgsLocalToLocal(t *testing.T) {
	args := BuildRsyncArgs(RsyncConfig{
		SourcePath:   "/data/source",
		SourceType:   "local",
		DestPath:     "/data/dest",
		DestType:     "local",
		LinkDestPath: "/data/prev",
		ExtraArgs:    []string{"--bwlimit=2048"},
	})

	want := []string{
		"-avz",
		"--delete",
		"--stats",
		"--info=progress2",
		"--link-dest=/data/prev",
		"--bwlimit=2048",
		"/data/source/",
		"/data/dest/",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("BuildRsyncArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildRsyncArgsLocalToSSH(t *testing.T) {
	remote := &model.RemoteConfig{
		Host:           "backup.example.com",
		Port:           2222,
		Username:       "rsync",
		PrivateKeyPath: "/keys/backup",
	}

	args := BuildRsyncArgs(RsyncConfig{
		SourcePath: "/data/source",
		SourceType: "local",
		DestPath:   "/srv/backups",
		DestType:   "ssh",
		DestRemote: remote,
	})

	want := []string{
		"-avz",
		"--delete",
		"--stats",
		"--info=progress2",
		"--rsh=ssh -i /keys/backup -p 2222 -o StrictHostKeyChecking=accept-new -o BatchMode=yes",
		"/data/source/",
		"rsync@backup.example.com:/srv/backups/",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("BuildRsyncArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildRsyncArgsSSHToLocal(t *testing.T) {
	remote := &model.RemoteConfig{
		Host:           "source.example.com",
		Port:           0,
		Username:       "reader",
		PrivateKeyPath: "/keys/source",
	}

	args := BuildRsyncArgs(RsyncConfig{
		SourcePath:       "/srv/data",
		SourceType:       "ssh",
		SourceRemote:     remote,
		DestPath:         "/data/restore",
		DestType:         "local",
		BandwidthLimitKB: 2048,
	})

	want := []string{
		"-avz",
		"--delete",
		"--stats",
		"--info=progress2",
		"--rsh=ssh -i /keys/source -p 22 -o StrictHostKeyChecking=accept-new -o BatchMode=yes",
		"--bwlimit=2048",
		"reader@source.example.com:/srv/data/",
		"/data/restore/",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("BuildRsyncArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildRsyncArgsDoesNotApplyBandwidthLimitOutsideSourcePull(t *testing.T) {
	remote := &model.RemoteConfig{
		Host:           "backup.example.com",
		Port:           2222,
		Username:       "rsync",
		PrivateKeyPath: "/keys/backup",
	}

	args := BuildRsyncArgs(RsyncConfig{
		SourcePath:       "/data/source",
		SourceType:       "local",
		DestPath:         "/srv/backups",
		DestType:         "ssh",
		DestRemote:       remote,
		BandwidthLimitKB: 2048,
	})

	for _, arg := range args {
		if arg == "--bwlimit=2048" {
			t.Fatalf("BuildRsyncArgs() unexpectedly included bandwidth limit: %#v", args)
		}
	}
}

func TestBuildRsyncArgsIncludesExcludePatterns(t *testing.T) {
	args := BuildRsyncArgs(RsyncConfig{
		SourcePath:      "/data/source",
		SourceType:      "local",
		DestPath:        "/data/dest",
		DestType:        "local",
		ExcludePatterns: []string{"*.log", "cache/**", "*.log", "node_modules/"},
	})

	want := []string{
		"-avz",
		"--delete",
		"--stats",
		"--info=progress2",
		"--exclude=*.log",
		"--exclude=cache/**",
		"--exclude=node_modules/",
		"/data/source/",
		"/data/dest/",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("BuildRsyncArgs() = %#v, want %#v", args, want)
	}
}

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   *ProgressInfo
		wantOK bool
	}{
		{
			name:   "raw bytes",
			line:   "  1,234,567  45%   12.34MB/s    0:01:23",
			want:   &ProgressInfo{BytesTransferred: 1234567, Percentage: 45, Speed: "12.34MB/s", Remaining: "0:01:23"},
			wantOK: true,
		},
		{
			name:   "human readable bytes",
			line:   "12.00K  45%   12.34MB/s    0:01:23",
			want:   &ProgressInfo{BytesTransferred: 12288, Percentage: 45, Speed: "12.34MB/s", Remaining: "0:01:23"},
			wantOK: true,
		},
		{
			name:   "invalid line",
			line:   "sending incremental file list",
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseProgress(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ParseProgress() ok = %v, want %v", ok, tt.wantOK)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseProgress() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseStats(t *testing.T) {
	stats := ParseStats(`Number of files: 1,234
Number of regular files transferred: 56
Total file size: 1,234,567 bytes
Total transferred file size: 123,456 bytes
sent 123,456 bytes  received 789 bytes  12,345.67 bytes/sec`)

	if stats.TotalFiles != 1234 {
		t.Fatalf("TotalFiles = %d, want %d", stats.TotalFiles, 1234)
	}
	if stats.TransferFiles != 56 {
		t.Fatalf("TransferFiles = %d, want %d", stats.TransferFiles, 56)
	}
	if stats.TotalSize != 1234567 {
		t.Fatalf("TotalSize = %d, want %d", stats.TotalSize, 1234567)
	}
	if stats.TransferSize != 123456 {
		t.Fatalf("TransferSize = %d, want %d", stats.TransferSize, 123456)
	}
	if stats.Speed != "12,345.67 bytes/sec" {
		t.Fatalf("Speed = %q, want %q", stats.Speed, "12,345.67 bytes/sec")
	}
}

func TestRsyncExecutorExecuteSuccess(t *testing.T) {
	executor := newHelperExecutor("success")
	var updates []ProgressInfo

	result, err := executor.Execute(context.Background(), RsyncConfig{
		SourcePath: "/data/source",
		SourceType: "local",
		DestPath:   "/data/dest",
		DestType:   "local",
	}, func(progress ProgressInfo) {
		updates = append(updates, progress)
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want %d", result.ExitCode, 0)
	}
	if len(updates) != 1 {
		t.Fatalf("progress updates = %d, want %d", len(updates), 1)
	}
	if updates[0].BytesTransferred != 1024 {
		t.Fatalf("BytesTransferred = %d, want %d", updates[0].BytesTransferred, 1024)
	}
	if result.Stats.TotalFiles != 10 || result.Stats.TransferFiles != 2 {
		t.Fatalf("Stats files = %#v, want total=10 transfer=2", result.Stats)
	}
	if result.Stats.TotalSize != 2048 || result.Stats.TransferSize != 1024 {
		t.Fatalf("Stats sizes = %#v, want total=2048 transfer=1024", result.Stats)
	}
	if result.Stats.Speed != "512.00 bytes/sec" {
		t.Fatalf("Speed = %q, want %q", result.Stats.Speed, "512.00 bytes/sec")
	}
	if result.Stats.Duration <= 0 {
		t.Fatalf("Duration = %s, want positive", result.Stats.Duration)
	}
}

func TestRsyncExecutorExecutePartialTransfer(t *testing.T) {
	executor := newHelperExecutor("partial")

	result, err := executor.Execute(context.Background(), RsyncConfig{
		SourcePath: "/data/source",
		SourceType: "local",
		DestPath:   "/data/dest",
		DestType:   "local",
	}, nil)
	if !errors.Is(err, ErrRsyncPartialTransfer) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrRsyncPartialTransfer)
	}
	if result == nil {
		t.Fatalf("Execute() result = nil, want non-nil")
	}
	if result.ExitCode != 23 {
		t.Fatalf("ExitCode = %d, want %d", result.ExitCode, 23)
	}
}

func TestRsyncExecutorExecuteCancel(t *testing.T) {
	executor := newHelperExecutor("hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := executor.Execute(ctx, RsyncConfig{
		SourcePath: "/data/source",
		SourceType: "local",
		DestPath:   "/data/dest",
		DestType:   "local",
	}, func(progress ProgressInfo) {
		if progress.Percentage == 10 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want %v", err, context.Canceled)
	}
	if result == nil {
		t.Fatalf("Execute() result = nil, want non-nil")
	}
	if result.Stdout == "" {
		t.Fatalf("Stdout = %q, want progress output", result.Stdout)
	}
	if result.Stats.Duration <= 0 {
		t.Fatalf("Duration = %s, want positive", result.Stats.Duration)
	}
}

func TestRsyncExecutorHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_RSYNC_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	mode := ""
	for index := range args {
		if args[index] == "--" && index+1 < len(args) {
			mode = args[index+1]
			break
		}
	}

	switch mode {
	case "success":
		fmt.Fprint(os.Stdout, "  1,024  10%   1.00MB/s    0:00:09\r")
		fmt.Fprintln(os.Stdout, "Number of files: 10")
		fmt.Fprintln(os.Stdout, "Number of regular files transferred: 2")
		fmt.Fprintln(os.Stdout, "Total file size: 2,048 bytes")
		fmt.Fprintln(os.Stdout, "Total transferred file size: 1,024 bytes")
		fmt.Fprintln(os.Stdout, "sent 1,024 bytes  received 128 bytes  512.00 bytes/sec")
		os.Exit(0)
	case "partial":
		fmt.Fprintln(os.Stderr, "some files vanished")
		os.Exit(23)
	case "hang":
		fmt.Fprint(os.Stdout, "  1,024  10%   1.00MB/s    0:00:09\r")
		for {
			time.Sleep(100 * time.Millisecond)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", mode)
		os.Exit(2)
	}
}

func newHelperExecutor(mode string) *RsyncExecutor {
	executor := NewRsyncExecutor()
	executor.commandName = os.Args[0]
	executor.newCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRsyncExecutorHelperProcess", "--", mode)
		cmd.Env = append(os.Environ(), "GO_WANT_RSYNC_HELPER_PROCESS=1")
		return cmd
	}
	return executor
}
