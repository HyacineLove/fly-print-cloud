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
	query := `
		INSERT INTO files (
			original_name, file_name, file_path, mime_type, size, uploader_id
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	
	return r.db.QueryRow(
		query,
		file.OriginalName,
		file.FileName,
		file.FilePath,
		file.MimeType,
		file.Size,
		file.UploaderID,
	).Scan(&file.ID, &file.CreatedAt)
}

func (r *FileRepository) GetByID(id string) (*models.File, error) {
	file := &models.File{}
	query := `
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, created_at
		FROM files WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&file.ID,
		&file.OriginalName,
		&file.FileName,
		&file.FilePath,
		&file.MimeType,
		&file.Size,
		&file.UploaderID,
		&file.CreatedAt,
	)
	
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
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, created_at
		FROM files WHERE created_at < $1`

	rows, err := r.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to list old files: %w", err)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		if err := rows.Scan(
			&file.ID,
			&file.OriginalName,
			&file.FileName,
			&file.FilePath,
			&file.MimeType,
			&file.Size,
			&file.UploaderID,
			&file.CreatedAt,
		); err != nil {
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
