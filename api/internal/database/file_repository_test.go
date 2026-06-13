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
		ContentHash:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	rows := sqlmock.NewRows([]string{"id", "created_at"}).AddRow("file-1", time.Unix(1, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO files (
			original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, content_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
			file.ContentHash,
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
		"id", "original_name", "file_name", "file_path", "mime_type", "size", "uploader_id", "storage_provider", "storage_bucket", "object_key", "content_hash", "created_at",
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
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		time.Unix(1, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, content_hash, created_at
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
	if file.ContentHash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("ContentHash = %q, want content hash", file.ContentHash)
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
		"id", "original_name", "file_name", "file_path", "mime_type", "size", "uploader_id", "storage_provider", "storage_bucket", "object_key", "content_hash", "created_at",
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
		nil,
		time.Unix(1, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, content_hash, created_at
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

func TestFileRepositoryListByStorageProvider(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewFileRepository(&DB{db})
	rows := sqlmock.NewRows([]string{
		"id", "original_name", "file_name", "file_path", "mime_type", "size", "uploader_id", "storage_provider", "storage_bucket", "object_key", "content_hash", "created_at",
	}).AddRow(
		"file-1",
		"report.pdf",
		"generated.pdf",
		"legacy/generated.pdf",
		"application/pdf",
		int64(1234),
		"user-1",
		"local",
		nil,
		"legacy/generated.pdf",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		time.Unix(1, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, original_name, file_name, file_path, mime_type, size, uploader_id, storage_provider, storage_bucket, object_key, content_hash, created_at
		FROM files
		WHERE COALESCE(NULLIF(storage_provider, ''), 'local') = $1
		ORDER BY created_at ASC`)).
		WithArgs("local").
		WillReturnRows(rows)

	files, err := repo.ListByStorageProvider("local")
	if err != nil {
		t.Fatalf("ListByStorageProvider() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("ListByStorageProvider() len = %d, want %d", len(files), 1)
	}
	if files[0].StorageProvider != "local" {
		t.Fatalf("StorageProvider = %q, want %q", files[0].StorageProvider, "local")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestFileRepositoryUpdateStorageMetadata(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewFileRepository(&DB{db})
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE files
		SET file_path = $2,
			storage_provider = $3,
			storage_bucket = $4,
			object_key = $5
		WHERE id = $1`)).
		WithArgs("file-1", "objects/report.pdf", "minio", "fly-print-files", "objects/report.pdf").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStorageMetadata("file-1", "minio", "fly-print-files", "objects/report.pdf"); err != nil {
		t.Fatalf("UpdateStorageMetadata() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
