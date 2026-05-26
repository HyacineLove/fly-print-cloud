package database

import (
	"regexp"
	"testing"
	"time"

	"fly-print-cloud/api/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFileRepositoryCreatePersistsObjectMetadata(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewFileRepository(&DB{db})
	file := &models.File{
		OriginalName:    "report.pdf",
		FileName:        "generated.pdf",
		FilePath:        "generated.pdf",
		MimeType:        "application/pdf",
		Size:            1234,
		UploaderID:      "user-1",
		StorageProvider: "minio",
		StorageBucket:   "fly-print-files",
		ObjectKey:       "uploads/generated.pdf",
	}

	rows := sqlmock.NewRows([]string{"id", "created_at"}).AddRow("file-1", time.Unix(1, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO files (
			original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`)).
		WithArgs(
			file.OriginalName,
			file.FileName,
			file.FilePath,
			file.MimeType,
			file.Size,
			file.UploaderID,
			file.StorageProvider,
			file.StorageBucket,
			file.ObjectKey,
		).
		WillReturnRows(rows)

	if err := repo.Create(file); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if file.ID != "file-1" {
		t.Fatalf("Create() id = %q, want %q", file.ID, "file-1")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestFileRepositoryGetByIDReadsObjectMetadata(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewFileRepository(&DB{db})
	rows := sqlmock.NewRows([]string{
		"id", "original_name", "file_name", "file_path", "mime_type", "size", "uploader_id", "storage_provider", "storage_bucket", "object_key", "created_at",
	}).AddRow(
		"file-1",
		"report.pdf",
		"generated.pdf",
		"generated.pdf",
		"application/pdf",
		int64(1234),
		"user-1",
		"minio",
		"fly-print-files",
		"uploads/generated.pdf",
		time.Unix(1, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, created_at
		FROM files WHERE id = $1`)).
		WithArgs("file-1").
		WillReturnRows(rows)

	file, err := repo.GetByID("file-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if file == nil {
		t.Fatalf("GetByID() file = nil, want non-nil")
	}
	if file.StorageProvider != "minio" {
		t.Fatalf("StorageProvider = %q, want %q", file.StorageProvider, "minio")
	}
	if file.StorageBucket != "fly-print-files" {
		t.Fatalf("StorageBucket = %q, want %q", file.StorageBucket, "fly-print-files")
	}
	if file.ObjectKey != "uploads/generated.pdf" {
		t.Fatalf("ObjectKey = %q, want %q", file.ObjectKey, "uploads/generated.pdf")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestFileRepositoryGetByIDFallsBackForLegacyRows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewFileRepository(&DB{db})
	rows := sqlmock.NewRows([]string{
		"id", "original_name", "file_name", "file_path", "mime_type", "size", "uploader_id", "storage_provider", "storage_bucket", "object_key", "created_at",
	}).AddRow(
		"file-1",
		"report.pdf",
		"generated.pdf",
		"legacy/generated.pdf",
		"application/pdf",
		int64(1234),
		"user-1",
		nil,
		nil,
		nil,
		time.Unix(1, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, created_at
		FROM files WHERE id = $1`)).
		WithArgs("file-1").
		WillReturnRows(rows)

	file, err := repo.GetByID("file-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if file == nil {
		t.Fatalf("GetByID() file = nil, want non-nil")
	}
	if file.StorageProvider != "local" {
		t.Fatalf("StorageProvider = %q, want %q", file.StorageProvider, "local")
	}
	if file.ObjectKey != "legacy/generated.pdf" {
		t.Fatalf("ObjectKey = %q, want %q", file.ObjectKey, "legacy/generated.pdf")
	}
	if file.FilePath != "legacy/generated.pdf" {
		t.Fatalf("FilePath = %q, want %q", file.FilePath, "legacy/generated.pdf")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
