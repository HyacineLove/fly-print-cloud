package handlers

import (
	"mime/multipart"
	"os"
	"testing"

	"fly-print-cloud/api/internal/config"
)

func TestValidateUploadRulesUsesConfiguredMaxSize(t *testing.T) {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "upload-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write([]byte("ab")); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek temp file: %v", err)
	}

	handler := &FileHandler{
		config: &config.StorageConfig{
			MaxSize: 1,
		},
	}

	fileHeader := &multipart.FileHeader{
		Filename: "resume.txt",
		Size:     2,
	}

	if err := handler.validateUploadRules(fileHeader, tmpFile); err != errUploadTooLarge {
		t.Fatalf("expected errUploadTooLarge, got %v", err)
	}
}
