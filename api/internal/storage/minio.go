package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"fly-print-cloud/api/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOBackend struct {
	client       *minio.Client
	bucket       string
	objectPrefix string
}

func NewMinIOBackend(cfg config.MinIOConfig) (*MinIOBackend, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return &MinIOBackend{
		client:       client,
		bucket:       cfg.Bucket,
		objectPrefix: strings.Trim(cfg.ObjectPrefix, "/"),
	}, nil
}

func (b *MinIOBackend) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*ObjectInfo, error) {
	objectKey, err := b.objectKey(key)
	if err != nil {
		return nil, err
	}

	uploadInfo, err := b.client.PutObject(ctx, b.bucket, objectKey, reader, -1, minio.PutObjectOptions{
		ContentType: opts.ContentType,
	})
	if err != nil {
		return nil, fmt.Errorf("put object to minio: %w", err)
	}

	return &ObjectInfo{
		Key:         key,
		Size:        uploadInfo.Size,
		ContentType: opts.ContentType,
		ModTime:     time.Now().UTC(),
	}, nil
}

func (b *MinIOBackend) Get(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error) {
	objectKey, err := b.objectKey(key)
	if err != nil {
		return nil, nil, err
	}

	object, err := b.client.GetObject(ctx, b.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get object from minio: %w", err)
	}

	stat, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, nil, fmt.Errorf("stat minio object: %w", err)
	}

	return object, &ObjectInfo{
		Key:         key,
		Size:        stat.Size,
		ContentType: stat.ContentType,
		ModTime:     stat.LastModified,
	}, nil
}

func (b *MinIOBackend) Delete(ctx context.Context, key string) error {
	objectKey, err := b.objectKey(key)
	if err != nil {
		return err
	}
	if err := b.client.RemoveObject(ctx, b.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil
		}
		return fmt.Errorf("delete minio object: %w", err)
	}
	return nil
}

func (b *MinIOBackend) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	objectKey, err := b.objectKey(key)
	if err != nil {
		return nil, err
	}

	info, err := b.client.StatObject(ctx, b.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:         key,
		Size:        info.Size,
		ContentType: info.ContentType,
		ModTime:     info.LastModified,
	}, nil
}

func (b *MinIOBackend) GeneratePresignedGet(ctx context.Context, key string, expiresIn time.Duration, downloadName string) (string, error) {
	objectKey, err := b.objectKey(key)
	if err != nil {
		return "", err
	}

	params := make(url.Values)
	if downloadName != "" {
		params.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
	}

	u, err := b.client.PresignedGetObject(ctx, b.bucket, objectKey, expiresIn, params)
	if err != nil {
		return "", fmt.Errorf("presign minio object: %w", err)
	}
	return u.String(), nil
}

func (b *MinIOBackend) objectKey(key string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(key), "/")
	if trimmed == "" {
		return "", fmt.Errorf("storage key is required")
	}
	if strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("storage key must not contain path traversal: %s", key)
	}
	if b.objectPrefix == "" {
		return trimmed, nil
	}
	return path.Join(b.objectPrefix, trimmed), nil
}
