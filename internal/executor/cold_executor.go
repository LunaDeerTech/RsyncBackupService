package executor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
)

type ColdBackupRequest struct {
	Instance            model.BackupInstance
	Target              model.StorageTarget
	Backend             storage.StorageBackend
	SourceSSHKeyPath    string
	VolumeSize          *string
	ArchivePath         string
	ArchiveRelativePath string
	TempRoot            string
	ExcludePatterns     []string
	Result              *ColdBackupResult
	Progress            func(ColdProgressSnapshot)
}

type ColdProgressSnapshot struct {
	Percentage int
}

type ColdBackupResult struct {
	ArchivePath      string
	VolumeCount      int
	BytesTransferred int64
	FilesTransferred int64
	TotalSize        int64
}

type ColdExecutor struct {
	runner Runner
}

func NewColdExecutor(runner Runner) *ColdExecutor {
	if runner == nil {
		runner = NewExecRunner()
	}

	return &ColdExecutor{runner: runner}
}

func (e *ColdExecutor) Run(ctx context.Context, req ColdBackupRequest) error {
	if e == nil {
		e = NewColdExecutor(nil)
	}
	if req.Backend == nil {
		return fmt.Errorf("storage backend is required")
	}
	if strings.TrimSpace(req.ArchivePath) == "" {
		return fmt.Errorf("archive path is required")
	}
	if strings.TrimSpace(req.ArchiveRelativePath) == "" {
		return fmt.Errorf("archive relative path is required")
	}
	emitColdProgress(req.Progress, 10)

	tempRoot := strings.TrimSpace(req.TempRoot)
	if isColdLocalTarget(req.Target) {
		tempRoot = filepath.Join(strings.TrimSpace(req.Target.BasePath), ".rbs-cold-tmp")
	}
	if tempRoot != "" {
		if err := os.MkdirAll(tempRoot, 0o755); err != nil {
			return fmt.Errorf("create cold backup temp root: %w", err)
		}
	}

	workDir, err := os.MkdirTemp(tempRoot, "rbs-cold-*")
	if err != nil {
		return fmt.Errorf("create cold backup workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	sourceDir := strings.TrimSpace(req.Instance.SourcePath)
	if isRemoteSource(req.Instance) {
		sourceDir = filepath.Join(workDir, "source")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			return fmt.Errorf("create staged source directory: %w", err)
		}
		if err := e.runner.Run(ctx, buildColdSourceStageCommand(req.Instance, req.SourceSSHKeyPath, sourceDir, req.ExcludePatterns), nil); err != nil {
			return err
		}
	}
	emitColdProgress(req.Progress, 35)

	totalSize, fileCount, err := directoryStats(sourceDir)
	if err != nil {
		return err
	}
	emitColdProgress(req.Progress, 50)

	archiveBase := filepath.Join(workDir, "archive")
	if err := e.runner.Run(ctx, BuildArchiveCommand(sourceDir, archiveBase, req.VolumeSize), nil); err != nil {
		return err
	}
	emitColdProgress(req.Progress, 75)

	archiveFiles, err := resolveArchiveFiles(archiveBase, req.VolumeSize)
	if err != nil {
		return err
	}

	var uploadedBytes int64
	uploadedPaths := make([]string, 0, len(archiveFiles))
	for index, archiveFile := range archiveFiles {
		archiveInfo, err := os.Stat(archiveFile)
		if err != nil {
			return fmt.Errorf("stat archive %q: %w", archiveFile, err)
		}

		remotePath, err := resolveArchiveRemotePath(archiveFile, archiveBase, req.ArchiveRelativePath)
		if err != nil {
			return err
		}
		if isColdLocalTarget(req.Target) {
			localTargetPath := filepath.Join(strings.TrimSpace(req.Target.BasePath), remotePath)
			if err := moveArchiveIntoLocalTarget(archiveFile, localTargetPath); err != nil {
				cleanupErr := cleanupUploadedArchives(ctx, req.Backend, append(uploadedPaths, remotePath))
				if cleanupErr != nil {
					return errors.Join(fmt.Errorf("move archive %q: %w", remotePath, err), cleanupErr)
				}
				return fmt.Errorf("move archive %q: %w", remotePath, err)
			}
		} else {
			if err := req.Backend.Upload(ctx, archiveFile, remotePath); err != nil {
				cleanupErr := cleanupUploadedArchives(ctx, req.Backend, append(uploadedPaths, remotePath))
				if cleanupErr != nil {
					return errors.Join(fmt.Errorf("upload archive %q: %w", remotePath, err), cleanupErr)
				}
				return fmt.Errorf("upload archive %q: %w", remotePath, err)
			}
		}
		uploadedPaths = append(uploadedPaths, remotePath)
		uploadedBytes += archiveInfo.Size()
		emitColdProgress(req.Progress, 75+int(float64(index+1)*20/float64(len(archiveFiles))))
	}

	if req.Result != nil {
		req.Result.ArchivePath = strings.TrimSpace(req.ArchivePath)
		req.Result.VolumeCount = len(archiveFiles)
		req.Result.BytesTransferred = uploadedBytes
		req.Result.FilesTransferred = fileCount
		req.Result.TotalSize = totalSize
	}

	return nil
}

func emitColdProgress(callback func(ColdProgressSnapshot), percentage int) {
	if callback == nil {
		return
	}
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 99 {
		percentage = 99
	}

	callback(ColdProgressSnapshot{Percentage: percentage})
}

func resolveArchiveRemotePath(archiveFile, archiveBase, archiveRelativePath string) (string, error) {
	archiveBaseFileName := filepath.Base(strings.TrimSpace(archiveBase)) + ".tar.gz"
	archiveFileName := filepath.Base(strings.TrimSpace(archiveFile))
	trimmedArchiveRelativePath := strings.TrimSpace(archiveRelativePath)
	if archiveFileName == archiveBaseFileName {
		return trimmedArchiveRelativePath, nil
	}
	if strings.HasPrefix(archiveFileName, archiveBaseFileName+".part_") {
		return trimmedArchiveRelativePath + strings.TrimPrefix(archiveFileName, archiveBaseFileName), nil
	}

	return "", fmt.Errorf("unexpected archive file %q for base %q", archiveFileName, archiveBaseFileName)
}

func isColdLocalTarget(target model.StorageTarget) bool {
	return strings.EqualFold(strings.TrimSpace(target.Type), "cold_local") && strings.TrimSpace(target.BasePath) != ""
}

func moveArchiveIntoLocalTarget(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create local archive directory: %w", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return fmt.Errorf("rename local archive into target: %w", err)
	}

	return nil
}

func buildColdSourceStageCommand(instance model.BackupInstance, sourceSSHKeyPath, destinationDir string, excludePatterns []string) CommandSpec {
	args := []string{"-a", "--delete", "--human-readable", "--info=progress2", "--protect-args"}
	for _, excludePattern := range excludePatterns {
		trimmedPattern := strings.TrimSpace(excludePattern)
		if trimmedPattern != "" {
			args = append(args, "--exclude", trimmedPattern)
		}
	}
	args = append(args, buildRemoteShellArgs(sourceSSHKeyPath, instance.SourcePort)...)
	args = append(args, withTrailingSlash(remoteLocation(instance.SourceUser, instance.SourceHost, instance.SourcePath)), withTrailingSlash(destinationDir))

	return CommandSpec{Name: "rsync", Args: args}
}

func resolveArchiveFiles(archiveBase string, volumeSize *string) ([]string, error) {
	if volumeSize == nil || strings.TrimSpace(*volumeSize) == "" {
		archivePath := archiveBase + ".tar.gz"
		if _, err := os.Stat(archivePath); err != nil {
			return nil, fmt.Errorf("stat archive %q: %w", archivePath, err)
		}
		return []string{archivePath}, nil
	}

	archiveParts, err := filepath.Glob(archiveBase + ".tar.gz.part_*")
	if err != nil {
		return nil, fmt.Errorf("glob archive parts: %w", err)
	}
	if len(archiveParts) == 0 {
		return nil, fmt.Errorf("archive parts were not created")
	}
	sort.Strings(archiveParts)
	return archiveParts, nil
}

func directoryStats(root string) (int64, int64, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return 0, 0, fmt.Errorf("source directory is required")
	}

	var totalSize int64
	var fileCount int64
	err := filepath.WalkDir(trimmedRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.Mode().IsRegular() {
			totalSize += entryInfo.Size()
			fileCount++
		}

		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("walk source directory %q: %w", trimmedRoot, err)
	}

	return totalSize, fileCount, nil
}

func cleanupUploadedArchives(ctx context.Context, backend storage.StorageBackend, paths []string) error {
	var cleanupErrors []error
	seenPaths := make(map[string]struct{}, len(paths))
	for index := len(paths) - 1; index >= 0; index-- {
		path := strings.TrimSpace(paths[index])
		if path == "" {
			continue
		}
		if _, exists := seenPaths[path]; exists {
			continue
		}
		seenPaths[path] = struct{}{}
		if err := backend.Delete(ctx, path); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete uploaded archive %q: %w", path, err))
		}
	}

	return errors.Join(cleanupErrors...)
}
