package storage

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestLocalStoragePutGetDelete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend, err := NewLocalBackend(root)
	if err != nil {
		t.Fatalf("NewLocalBackend() error = %v", err)
	}

	key := filepath.ToSlash(filepath.Join("nested", "sample.txt"))
	payload := []byte("hello from local storage")

	info, err := backend.Put(context.Background(), key, bytes.NewReader(payload), PutOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != key {
		t.Fatalf("Put() key = %q, want %q", info.Key, key)
	}
	if info.Size != int64(len(payload)) {
		t.Fatalf("Put() size = %d, want %d", info.Size, len(payload))
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("Put() content type = %q, want %q", info.ContentType, "text/plain")
	}

	stat, err := backend.Stat(context.Background(), key)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if stat.Key != key {
		t.Fatalf("Stat() key = %q, want %q", stat.Key, key)
	}
	if stat.Size != int64(len(payload)) {
		t.Fatalf("Stat() size = %d, want %d", stat.Size, len(payload))
	}

	reader, meta, err := backend.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Get() payload = %q, want %q", string(got), string(payload))
	}
	if meta.Key != key {
		t.Fatalf("Get() key = %q, want %q", meta.Key, key)
	}
	if meta.Size != int64(len(payload)) {
		t.Fatalf("Get() size = %d, want %d", meta.Size, len(payload))
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := backend.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := backend.Stat(context.Background(), key); err == nil {
		t.Fatalf("Stat() after Delete() error = nil, want non-nil")
	}
}

func TestLocalStorageSupportsLegacyRootPrefixedKeys(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend, err := NewLocalBackend(root)
	if err != nil {
		t.Fatalf("NewLocalBackend() error = %v", err)
	}

	legacyPath := filepath.Join(root, "legacy.txt")
	if _, err := backend.Put(context.Background(), legacyPath, bytes.NewReader([]byte("legacy")), PutOptions{}); err != nil {
		t.Fatalf("Put() with legacy path error = %v", err)
	}

	reader, _, err := backend.Get(context.Background(), legacyPath)
	if err != nil {
		t.Fatalf("Get() with legacy path error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != "legacy" {
		t.Fatalf("Get() payload = %q, want %q", string(got), "legacy")
	}
}
