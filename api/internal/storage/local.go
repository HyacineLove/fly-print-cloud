package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalBackend struct {
	root string
}

func NewLocalBackend(root string) (*LocalBackend, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("local storage root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	return &LocalBackend{root: root}, nil
}

func (b *LocalBackend) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := b.resolvePath(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create object directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create local object: %w", err)
	}

	size, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write local object: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close local object: %w", closeErr)
	}

	return &ObjectInfo{
		Key:         key,
		Size:        size,
		ContentType: opts.ContentType,
		ModTime:     time.Now().UTC(),
	}, nil
}

func (b *LocalBackend) Get(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	path, err := b.resolvePath(key)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("stat local object: %w", err)
	}

	return file, buildLocalObjectInfo(key, info), nil
}

func (b *LocalBackend) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := b.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local object: %w", err)
	}
	return nil
}

func (b *LocalBackend) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := b.resolvePath(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return buildLocalObjectInfo(key, info), nil
}

func (b *LocalBackend) GeneratePresignedGet(context.Context, string, time.Duration, string) (string, error) {
	return "", ErrNotSupported
}

func (b *LocalBackend) resolvePath(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("storage key is required")
	}

	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanKey == "." || cleanKey == "" {
		return "", fmt.Errorf("storage key is required")
	}
	if filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("absolute storage keys are not allowed")
	}
	if cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key escapes root: %s", key)
	}

	fullPath := filepath.Join(b.root, cleanKey)
	rel, err := filepath.Rel(b.root, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve storage key: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key escapes root: %s", key)
	}
	return fullPath, nil
}

func buildLocalObjectInfo(key string, info os.FileInfo) *ObjectInfo {
	return &ObjectInfo{
		Key:         key,
		Size:        info.Size(),
		ContentType: mime.TypeByExtension(filepath.Ext(key)),
		ModTime:     info.ModTime(),
	}
}
