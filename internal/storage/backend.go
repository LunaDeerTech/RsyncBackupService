package storage

import "context"

type StorageObject struct {
	Path       string
	Size       int64
	IsDir      bool
	ModifiedAt int64
}

type StorageBackend interface {
	Type() string
	Upload(ctx context.Context, localPath, remotePath string) error
	Download(ctx context.Context, remotePath, localPath string) error
	List(ctx context.Context, prefix string) ([]StorageObject, error)
	Delete(ctx context.Context, path string) error
	SpaceAvailable(ctx context.Context, path string) (uint64, error)
	TestConnection(ctx context.Context) error
}