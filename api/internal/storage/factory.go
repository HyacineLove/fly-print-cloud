package storage

import (
	"fmt"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/models"
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

func NewForFile(cfg config.StorageConfig, file *models.File) (Service, error) {
	effective := cfg
	if file != nil && file.StorageProvider != "" {
		effective.Provider = file.StorageProvider
	}
	if effective.Provider == "" {
		effective.Provider = "local"
	}
	return NewFromConfig(effective)
}
