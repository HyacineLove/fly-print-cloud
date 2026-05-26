package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotSupported = errors.New("storage operation not supported")

type PutOptions struct {
	ContentType string
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ModTime     time.Time
}

type Service interface {
	Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*ObjectInfo, error)
	Get(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (*ObjectInfo, error)
	GeneratePresignedGet(ctx context.Context, key string, expiresIn time.Duration, downloadName string) (string, error)
}
