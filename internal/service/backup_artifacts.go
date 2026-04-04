package service

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
)

func listStorageObjectsWithPrefix(ctx context.Context, backend storage.StorageBackend, relativePrefix string) ([]string, error) {
	if backend == nil {
		return nil, nil
	}

	cleanRelativePrefix := filepath.Clean(strings.TrimSpace(relativePrefix))
	if cleanRelativePrefix == "." || cleanRelativePrefix == "" {
		return nil, nil
	}

	parentPath := filepath.Dir(cleanRelativePrefix)
	if parentPath == "" {
		parentPath = "."
	}

	objects, err := backend.List(ctx, parentPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	prefixBase := filepath.Base(cleanRelativePrefix)
	matchingPaths := make([]string, 0, len(objects))
	for _, object := range objects {
		objectPath := filepath.Clean(strings.TrimSpace(object.Path))
		if strings.HasPrefix(filepath.Base(objectPath), prefixBase) {
			matchingPaths = append(matchingPaths, objectPath)
		}
	}
	sort.Strings(matchingPaths)

	return matchingPaths, nil
}

func storageObjectExists(ctx context.Context, backend storage.StorageBackend, relativePath string) (bool, error) {
	if backend == nil {
		return false, nil
	}

	cleanRelativePath := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return false, nil
	}

	parentPath := filepath.Dir(cleanRelativePath)
	if parentPath == "" {
		parentPath = "."
	}

	objects, err := backend.List(ctx, parentPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	baseName := filepath.Base(cleanRelativePath)
	for _, object := range objects {
		if filepath.Base(object.Path) == baseName {
			return true, nil
		}
	}

	return false, nil
}

func storageObjectMetadata(ctx context.Context, backend storage.StorageBackend, relativePath string) (storage.StorageObject, bool, error) {
	if backend == nil {
		return storage.StorageObject{}, false, nil
	}
	if err := ctx.Err(); err != nil {
		return storage.StorageObject{}, false, err
	}

	cleanRelativePath := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return storage.StorageObject{}, false, nil
	}

	parentPath := filepath.Dir(cleanRelativePath)
	if parentPath == "" {
		parentPath = "."
	}

	objects, err := backend.List(ctx, parentPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.StorageObject{}, false, nil
		}
		return storage.StorageObject{}, false, err
	}

	baseName := filepath.Base(cleanRelativePath)
	for _, object := range objects {
		if filepath.Base(object.Path) == baseName {
			return object, true, nil
		}
	}

	return storage.StorageObject{}, false, nil
}

func storageObjectTotalSize(ctx context.Context, backend storage.StorageBackend, relativePath string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}

	object, exists, err := storageObjectMetadata(ctx, backend, relativePath)
	if err != nil || !exists {
		return 0, exists, err
	}

	totalSize, err := storageObjectTotalSizeFromObject(ctx, backend, object)
	if err != nil {
		return 0, false, err
	}

	return totalSize, true, nil
}

func storageObjectTotalSizeFromObject(ctx context.Context, backend storage.StorageBackend, object storage.StorageObject) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !object.IsDir {
		return object.Size, nil
	}

	children, err := backend.List(ctx, filepath.Clean(strings.TrimSpace(object.Path)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	var totalSize int64
	for _, child := range children {
		childSize, err := storageObjectTotalSizeFromObject(ctx, backend, child)
		if err != nil {
			return 0, err
		}
		totalSize += childSize
	}

	return totalSize, nil
}

func resolveColdArchiveObjectPaths(ctx context.Context, backend storage.StorageBackend, archiveRelativePath string, volumeCount int) ([]string, error) {
	exists, err := storageObjectExists(ctx, backend, archiveRelativePath)
	if err != nil {
		return nil, err
	}
	if exists {
		return []string{archiveRelativePath}, nil
	}

	splitParts, err := listStorageObjectsWithPrefix(ctx, backend, archiveRelativePath+".part_")
	if err != nil {
		return nil, err
	}
	if volumeCount > 1 {
		if len(splitParts) != volumeCount {
			return nil, ErrBackupRecordNotRestorable
		}
		return splitParts, nil
	}
	if len(splitParts) == 1 {
		return splitParts, nil
	}

	return nil, ErrBackupRecordNotRestorable
}

func backupRecordArtifactsExist(ctx context.Context, backend storage.StorageBackend, record model.BackupRecord, target model.StorageTarget) (bool, error) {
	relativePath, ok := relativeTargetPath(record.SnapshotPath, target.BasePath)
	if !ok {
		return false, nil
	}

	if strings.EqualFold(strings.TrimSpace(record.BackupType), BackupTypeCold) {
		_, err := resolveColdArchiveObjectPaths(ctx, backend, relativePath, record.VolumeCount)
		if err != nil {
			if errors.Is(err, ErrBackupRecordNotRestorable) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	return storageObjectExists(ctx, backend, relativePath)
}

func backupRecordArtifactSize(ctx context.Context, backend storage.StorageBackend, record model.BackupRecord, target model.StorageTarget) (int64, bool, error) {
	relativePath, ok := relativeTargetPath(record.SnapshotPath, target.BasePath)
	if !ok {
		return 0, false, nil
	}

	if strings.EqualFold(strings.TrimSpace(record.BackupType), BackupTypeCold) {
		archivePaths, err := resolveColdArchiveObjectPaths(ctx, backend, relativePath, record.VolumeCount)
		if err != nil {
			if errors.Is(err, ErrBackupRecordNotRestorable) {
				return 0, false, nil
			}
			return 0, false, err
		}

		var totalSize int64
		for _, archivePath := range archivePaths {
			currentSize, exists, err := storageObjectTotalSize(ctx, backend, archivePath)
			if err != nil {
				return 0, false, err
			}
			if !exists {
				return 0, false, nil
			}
			totalSize += currentSize
		}

		return totalSize, true, nil
	}

	return storageObjectTotalSize(ctx, backend, relativePath)
}