package database

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSystemSettingsRepositoryGetValues(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewSystemSettingsRepository(&DB{db})
	rows := sqlmock.NewRows([]string{"key", "value"}).
		AddRow("upload.max_size_bytes", "1048576").
		AddRow("upload.max_document_pages", "3")

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT key, value
		FROM system_settings
		WHERE key = ANY($1)`)).
		WillReturnRows(rows)

	values, err := repo.GetValues([]string{"upload.max_size_bytes", "upload.max_document_pages"})
	if err != nil {
		t.Fatalf("GetValues() error = %v", err)
	}
	if values["upload.max_size_bytes"] != "1048576" {
		t.Fatalf("upload.max_size_bytes = %q, want %q", values["upload.max_size_bytes"], "1048576")
	}
	if values["upload.max_document_pages"] != "3" {
		t.Fatalf("upload.max_document_pages = %q, want %q", values["upload.max_document_pages"], "3")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestSystemSettingsRepositorySetValues(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewSystemSettingsRepository(&DB{db})
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO system_settings (key, value)
			VALUES ($1, $2)
			ON CONFLICT (key) DO UPDATE SET
				value = EXCLUDED.value,
				updated_at = CURRENT_TIMESTAMP`)).
		WithArgs("upload.max_size_bytes", "1048576").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.SetValues(map[string]string{"upload.max_size_bytes": "1048576"}); err != nil {
		t.Fatalf("SetValues() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
