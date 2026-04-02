package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
)

func TestColdExecutorMovesArchiveDirectlyForLocalTarget(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "db.dump"), []byte("backup"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	targetBasePath := filepath.Join(t.TempDir(), "target")
	backend := &coldExecutorStorageSpy{}
	archiveRelativePath := filepath.Join("instance-1", "archive.tar.gz")
	archivePath := filepath.Join(targetBasePath, archiveRelativePath)

	err := NewColdExecutor(coldExecutorRunnerFunc(func(_ context.Context, spec CommandSpec, _ func(string)) error {
		if spec.Name == "tar" && len(spec.Args) >= 2 && spec.Args[0] == "czf" {
			if err := os.MkdirAll(filepath.Dir(spec.Args[1]), 0o755); err != nil {
				return err
			}
			return os.WriteFile(spec.Args[1], []byte("archive"), 0o644)
		}
		return nil
	})).Run(context.Background(), ColdBackupRequest{
		Instance: model.BackupInstance{
			SourceType: "local",
			SourcePath: sourceDir,
		},
		Target: model.StorageTarget{
			Type:     "cold_local",
			BasePath: targetBasePath,
		},
		Backend:             backend,
		ArchivePath:         archivePath,
		ArchiveRelativePath: archiveRelativePath,
	})
	if err != nil {
		t.Fatalf("run cold executor: %v", err)
	}
	if backend.uploadCalls != 0 {
		t.Fatalf("expected cold_local target to avoid backend upload, got %d upload calls", backend.uploadCalls)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected moved archive at %q: %v", archivePath, err)
	}
}

func TestColdExecutorPreservesActualSplitSuffixesWhenUploading(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "db.dump"), []byte("backup"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	backend := &coldExecutorStorageSpy{}
	archiveRelativePath := filepath.Join("instance-1", "archive.tar.gz")
	volumeSize := "10M"
	actualArchiveNames := []string{
		"archive.tar.gz.part_yz",
		"archive.tar.gz.part_zaaa",
		"archive.tar.gz.part_zaab",
	}

	err := NewColdExecutor(coldExecutorRunnerFunc(func(_ context.Context, spec CommandSpec, _ func(string)) error {
		if spec.Name != "sh" {
			return nil
		}
		commandParts := strings.Split(spec.Args[1], "'")
		if len(commandParts) < 2 {
			return nil
		}
		archivePrefixPath := commandParts[len(commandParts)-2]
		for _, archiveName := range actualArchiveNames {
			archivePath := filepath.Join(filepath.Dir(archivePrefixPath), archiveName)
			if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(archivePath, []byte("archive"), 0o644); err != nil {
				return err
			}
		}
		return nil
	})).Run(context.Background(), ColdBackupRequest{
		Instance: model.BackupInstance{
			SourceType: "local",
			SourcePath: sourceDir,
		},
		Target: model.StorageTarget{
			Type:     "cold_ssh",
			BasePath: "/srv/backups",
		},
		Backend:             backend,
		ArchivePath:         filepath.Join(t.TempDir(), "unused", "archive.tar.gz"),
		ArchiveRelativePath: archiveRelativePath,
		VolumeSize:          &volumeSize,
	})
	if err != nil {
		t.Fatalf("run cold executor: %v", err)
	}

	expectedUploads := []string{
		filepath.Join("instance-1", "archive.tar.gz.part_yz"),
		filepath.Join("instance-1", "archive.tar.gz.part_zaaa"),
		filepath.Join("instance-1", "archive.tar.gz.part_zaab"),
	}
	if len(backend.uploadPaths) != len(expectedUploads) {
		t.Fatalf("expected %d upload paths, got %d", len(expectedUploads), len(backend.uploadPaths))
	}
	for index := range expectedUploads {
		if backend.uploadPaths[index] != expectedUploads[index] {
			t.Fatalf("expected upload path %q at index %d, got %q", expectedUploads[index], index, backend.uploadPaths[index])
		}
	}
}

type coldExecutorRunnerFunc func(context.Context, CommandSpec, func(string)) error

func (f coldExecutorRunnerFunc) Run(ctx context.Context, spec CommandSpec, onStdout func(string)) error {
	return f(ctx, spec, onStdout)
}

type coldExecutorStorageSpy struct {
	uploadCalls int
	uploadPaths []string
}

func (s *coldExecutorStorageSpy) Type() string {
	return "spy"
}

func (s *coldExecutorStorageSpy) Upload(_ context.Context, _ string, remotePath string) error {
	s.uploadCalls++
	s.uploadPaths = append(s.uploadPaths, remotePath)
	return nil
}

func (s *coldExecutorStorageSpy) Download(context.Context, string, string) error {
	return nil
}

func (s *coldExecutorStorageSpy) List(context.Context, string) ([]storage.StorageObject, error) {
	return nil, nil
}

func (s *coldExecutorStorageSpy) Delete(context.Context, string) error {
	return nil
}

func (s *coldExecutorStorageSpy) SpaceAvailable(context.Context, string) (uint64, error) {
	return 0, nil
}

func (s *coldExecutorStorageSpy) TestConnection(context.Context) error {
	return nil
}