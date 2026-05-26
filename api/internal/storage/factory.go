package storage

import (
	"fmt"

	"fly-print-cloud/api/internal/config"
)

func NewFromConfig(cfg config.StorageConfig) (Service, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalBackend(cfg.UploadDir)
	case "minio":
		return NewMinIOBackend(cfg.MinIO)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}
