package database

import (
	"database/sql"
	"fmt"
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
