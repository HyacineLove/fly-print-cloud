package database

import (
	"database/sql"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"
)

type FileRepository struct {
	db *DB
}

func NewFileRepository(db *DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(file *models.File) error {
	provider := file.StorageProvider
	if provider == "" {
		provider = "local"
	}
	objectKey := file.ObjectKey
	if objectKey == "" {
		objectKey = file.FilePath
	}
	if file.FilePath == "" {
		file.FilePath = objectKey
	}
	file.StorageProvider = provider
	file.ObjectKey = objectKey

	query := `
		INSERT INTO files (
			original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	return r.db.QueryRow(
		query,
		file.OriginalName,
		file.FileName,
		file.FilePath,
		file.MimeType,
		file.Size,
		file.UploaderID,
		file.StorageProvider,
		file.StorageBucket,
		file.ObjectKey,
	).Scan(&file.ID, &file.CreatedAt)
}

func (r *FileRepository) GetByID(id string) (*models.File, error) {
	query := `
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, created_at
		FROM files WHERE id = $1`

	file, err := scanFileRow(r.db.QueryRow(query, id))

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return file, nil
}

// ListOldFiles 列出早于指定时间创建的文件，用于清理任务
func (r *FileRepository) ListOldFiles(cutoff time.Time) ([]*models.File, error) {
	query := `
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, created_at
		FROM files WHERE created_at < $1`

	rows, err := r.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to list old files: %w", err)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file, err := scanFileRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan old file: %w", err)
		}
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return files, nil
}

// DeleteByID 根据ID删除文件记录
func (r *FileRepository) DeleteByID(id string) error {
	query := `DELETE FROM files WHERE id = $1`
	if _, err := r.db.Exec(query, id); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (r *FileRepository) ListByStorageProvider(provider string) ([]*models.File, error) {
	query := `
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, created_at
		FROM files
		WHERE COALESCE(NULLIF(storage_provider, ''), 'local') = $1
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list files by storage provider: %w", err)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file, err := scanFileRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file by storage provider: %w", err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return files, nil
}

func (r *FileRepository) UpdateStorageMetadata(id, provider, bucket, objectKey string) error {
	query := `
		UPDATE files
		SET file_path = $2,
			storage_provider = $3,
			storage_bucket = $4,
			object_key = $5
		WHERE id = $1`

	if _, err := r.db.Exec(query, id, objectKey, provider, bucket, objectKey); err != nil {
		return fmt.Errorf("failed to update file storage metadata: %w", err)
	}
	return nil
}

type fileScanner interface {
	Scan(dest ...any) error
}

func scanFileRow(scanner fileScanner) (*models.File, error) {
	file := &models.File{}
	var storageProvider sql.NullString
	var storageBucket sql.NullString
	var objectKey sql.NullString

	if err := scanner.Scan(
		&file.ID,
		&file.OriginalName,
		&file.FileName,
		&file.FilePath,
		&file.MimeType,
		&file.Size,
		&file.UploaderID,
		&storageProvider,
		&storageBucket,
		&objectKey,
		&file.CreatedAt,
	); err != nil {
		return nil, err
	}

	if storageProvider.Valid && storageProvider.String != "" {
		file.StorageProvider = storageProvider.String
	} else {
		file.StorageProvider = "local"
	}
	if storageBucket.Valid {
		file.StorageBucket = storageBucket.String
	}
	if objectKey.Valid && objectKey.String != "" {
		file.ObjectKey = objectKey.String
	} else {
		file.ObjectKey = file.FilePath
	}
	if file.FilePath == "" {
		file.FilePath = file.ObjectKey
	}

	return file, nil
}
