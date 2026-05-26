package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/logger"
	"fly-print-cloud/api/internal/storage"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config error: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.App.Debug); err != nil {
		fmt.Fprintf(os.Stderr, "init logger error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	fileRepo := database.NewFileRepository(db)

	sourceStorage, err := storage.NewLocalBackend(cfg.Storage.UploadDir)
	if err != nil {
		logger.Fatal("Failed to initialize local storage backend", zap.Error(err))
	}

	targetCfg := cfg.Storage
	targetCfg.Provider = "minio"
	targetStorage, err := storage.NewFromConfig(targetCfg)
	if err != nil {
		logger.Fatal("Failed to initialize target storage backend", zap.Error(err))
	}

	files, err := fileRepo.ListByStorageProvider("local")
	if err != nil {
		logger.Fatal("Failed to list local-backed files", zap.Error(err))
	}

	var migratedCount int
	var failedCount int

	for _, file := range files {
		sourceKey := file.ObjectKey
		if sourceKey == "" {
			sourceKey = file.FilePath
		}

		reader, _, err := sourceStorage.Get(context.Background(), sourceKey)
		if err != nil {
			failedCount++
			logger.Warn("Backfill: failed to read source file", zap.String("file_id", file.ID), zap.String("source_key", sourceKey), zap.Error(err))
			continue
		}

		targetKey := file.FileName
		if targetKey == "" {
			targetKey = filepath.Base(sourceKey)
		}

		if _, err := targetStorage.Put(context.Background(), targetKey, reader, storage.PutOptions{
			ContentType: file.MimeType,
		}); err != nil {
			_ = reader.Close()
			failedCount++
			logger.Warn("Backfill: failed to write target file", zap.String("file_id", file.ID), zap.String("target_key", targetKey), zap.Error(err))
			continue
		}
		_ = reader.Close()

		if err := fileRepo.UpdateStorageMetadata(file.ID, "minio", cfg.Storage.MinIO.Bucket, targetKey); err != nil {
			failedCount++
			logger.Warn("Backfill: failed to update file metadata", zap.String("file_id", file.ID), zap.String("target_key", targetKey), zap.Error(err))
			continue
		}

		migratedCount++
		logger.Info("Backfill: migrated file", zap.String("file_id", file.ID), zap.String("target_key", targetKey))
	}

	logger.Info("Backfill completed", zap.Int("migrated", migratedCount), zap.Int("failed", failedCount), zap.Int("total", len(files)))

	if failedCount > 0 {
		os.Exit(1)
	}
}
