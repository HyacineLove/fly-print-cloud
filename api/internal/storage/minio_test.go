package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"fly-print-cloud/api/internal/config"
)

func TestMinIOBackendObjectLifecycle(t *testing.T) {
	t.Parallel()

	cfg, ok := loadMinIOTestConfig(t)
	if !ok {
		t.Skip("MinIO test environment is not configured")
	}

	backend, err := NewMinIOBackend(cfg)
	if err != nil {
		t.Fatalf("NewMinIOBackend() error = %v", err)
	}

	key := "codex-tests/" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".txt"
	payload := []byte("hello from minio storage")

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
	got, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Get() payload = %q, want %q", string(got), string(payload))
	}
	if meta.Key != key {
		t.Fatalf("Get() key = %q, want %q", meta.Key, key)
	}

	url, err := backend.GeneratePresignedGet(context.Background(), key, time.Minute, "download.txt")
	if err != nil {
		t.Fatalf("GeneratePresignedGet() error = %v", err)
	}
	if !strings.Contains(url, key) {
		t.Fatalf("GeneratePresignedGet() = %q, want URL containing key %q", url, key)
	}

	if err := backend.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := backend.Stat(context.Background(), key); err == nil {
		t.Fatalf("Stat() after Delete() error = nil, want non-nil")
	}
}

func loadMinIOTestConfig(t *testing.T) (config.MinIOConfig, bool) {
	t.Helper()

	cfg := config.MinIOConfig{
		Endpoint:     os.Getenv("FLY_PRINT_TEST_MINIO_ENDPOINT"),
		AccessKey:    os.Getenv("FLY_PRINT_TEST_MINIO_ACCESS_KEY"),
		SecretKey:    os.Getenv("FLY_PRINT_TEST_MINIO_SECRET_KEY"),
		Bucket:       os.Getenv("FLY_PRINT_TEST_MINIO_BUCKET"),
		ObjectPrefix: os.Getenv("FLY_PRINT_TEST_MINIO_OBJECT_PREFIX"),
	}

	if v := os.Getenv("FLY_PRINT_TEST_MINIO_USE_SSL"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			t.Fatalf("ParseBool(%q) error = %v", v, err)
		}
		cfg.UseSSL = parsed
	}

	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return config.MinIOConfig{}, false
	}
	return cfg, true
}
