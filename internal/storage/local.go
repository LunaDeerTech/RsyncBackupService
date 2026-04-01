package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: filepath.Clean(basePath)}
}

func (s *LocalStorage) Type() string {
	return "local"
}

func (s *LocalStorage) Upload(ctx context.Context, localPath, remotePath string) error {
	resolvedPath, err := s.resolvePath(remotePath)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer inputFile.Close()

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return fmt.Errorf("create local storage directory: %w", err)
	}

	outputFile, err := os.Create(resolvedPath)
	if err != nil {
		return fmt.Errorf("create target file: %w", err)
	}
	defer outputFile.Close()

	if _, err := copyWithContext(ctx, outputFile, inputFile); err != nil {
		return fmt.Errorf("upload file to local storage: %w", err)
	}

	return nil
}

func (s *LocalStorage) Download(ctx context.Context, remotePath, localPath string) error {
	resolvedPath, err := s.resolvePath(remotePath)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(resolvedPath)
	if err != nil {
		return fmt.Errorf("open local storage file: %w", err)
	}
	defer inputFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}

	outputFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create downloaded file: %w", err)
	}
	defer outputFile.Close()

	if _, err := copyWithContext(ctx, outputFile, inputFile); err != nil {
		return fmt.Errorf("download file from local storage: %w", err)
	}

	return nil
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]StorageObject, error) {
	resolvedPath, err := s.resolvePath(prefix)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("list local storage path: %w", err)
	}

	objects := make([]StorageObject, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read local storage entry info: %w", err)
		}

		objects = append(objects, StorageObject{
			Path:       filepath.Join(strings.TrimSpace(prefix), entry.Name()),
			Size:       entryInfo.Size(),
			IsDir:      entry.IsDir(),
			ModifiedAt: entryInfo.ModTime().UTC().Unix(),
		})
	}

	return objects, nil
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(resolvedPath); err != nil {
		return fmt.Errorf("delete local storage path: %w", err)
	}

	return nil
}

func (s *LocalStorage) SpaceAvailable(ctx context.Context, path string) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(resolvedPath, 0o755); err != nil {
		return 0, fmt.Errorf("create local storage path for statfs: %w", err)
	}

	var statfs syscall.Statfs_t
	if err := syscall.Statfs(resolvedPath, &statfs); err != nil {
		return 0, fmt.Errorf("stat local storage path: %w", err)
	}

	return statfs.Bavail * uint64(statfs.Bsize), nil
}

func (s *LocalStorage) TestConnection(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(s.basePath) == "" {
		return fmt.Errorf("local storage base path is required")
	}

	if err := os.MkdirAll(s.basePath, 0o755); err != nil {
		return fmt.Errorf("ensure local storage base path: %w", err)
	}

	if _, err := s.SpaceAvailable(ctx, "."); err != nil {
		return err
	}

	return nil
}

func (s *LocalStorage) resolvePath(candidate string) (string, error) {
	if strings.TrimSpace(s.basePath) == "" {
		return "", fmt.Errorf("local storage base path is required")
	}

	resolvedBasePath := filepath.Clean(s.basePath)
	resolvedPath := resolvedBasePath
	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedCandidate != "" && trimmedCandidate != "." {
		resolvedPath = filepath.Clean(filepath.Join(resolvedBasePath, trimmedCandidate))
	}

	if resolvedPath != resolvedBasePath && !strings.HasPrefix(resolvedPath, resolvedBasePath+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes local storage base path", candidate)
	}

	return resolvedPath, nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64

	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}

		readBytes, readErr := source.Read(buffer)
		if readBytes > 0 {
			writtenBytes, writeErr := destination.Write(buffer[:readBytes])
			written += int64(writtenBytes)
			if writeErr != nil {
				return written, writeErr
			}
			if writtenBytes != readBytes {
				return written, io.ErrShortWrite
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}