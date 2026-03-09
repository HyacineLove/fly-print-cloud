package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/security"

	"github.com/gin-gonic/gin"
)

type countingTokenRepo struct {
	markCalls int
}

func (m *countingTokenRepo) PreRegisterToken(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	return nil
}

func (m *countingTokenRepo) MarkTokenAsUsed(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	m.markCalls++
	return nil
}

func createMultipartRequest(path string, fileName string, content []byte) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func newTestFileHandler(t *testing.T, tokenManager *security.TokenManager) *FileHandler {
	t.Helper()
	uploadDir := filepath.Join(t.TempDir(), "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatalf("failed to create upload dir: %v", err)
	}
	return NewFileHandler(nil, &config.StorageConfig{
		UploadDir:        uploadDir,
		MaxSize:          uploadRuleMaxSizeBytes,
		MaxDocumentPages: uploadRuleMaxPages,
	}, nil, tokenManager)
}

func parseJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return result
}

func buildDocxWithPages(t *testing.T, pages int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	fileWriter, err := zipWriter.Create("docProps/app.xml")
	if err != nil {
		t.Fatalf("failed to create app.xml: %v", err)
	}
	content := `<?xml version="1.0" encoding="UTF-8"?><Properties><Pages>` + strconv.Itoa(pages) + `</Pages></Properties>`
	if _, err := fileWriter.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write app.xml: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return buf.Bytes()
}

func TestPreflightUpload_DoesNotConsumeToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenRepo := &countingTokenRepo{}
	tokenManager := security.NewTokenManager("test-secret", 180, 180, tokenRepo)
	handler := newTestFileHandler(t, tokenManager)

	token, _, err := tokenManager.GenerateUploadToken("node-1", "printer-1")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req, err := createMultipartRequest("/api/v1/files/preflight?token="+token, "ok.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	handler.PreflightUpload(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if tokenRepo.markCalls != 0 {
		t.Fatalf("expected preflight not consume token, mark calls=%d", tokenRepo.markCalls)
	}

	if _, err := tokenManager.ValidateUploadToken(token); err != nil {
		t.Fatalf("expected token still valid after preflight: %v", err)
	}
	if tokenRepo.markCalls != 1 {
		t.Fatalf("expected one consume call after validate, got %d", tokenRepo.markCalls)
	}
}

func TestPreflightUpload_RejectsFileLargerThan10MB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenManager := security.NewTokenManager("test-secret", 180, 180, nil)
	handler := newTestFileHandler(t, tokenManager)
	token, _, _ := tokenManager.GenerateUploadToken("node-1", "printer-1")

	largeContent := bytes.Repeat([]byte("a"), uploadRuleMaxSizeBytes+1)
	req, err := createMultipartRequest("/api/v1/files/preflight?token="+token, "large.txt", largeContent)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	handler.PreflightUpload(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	result := parseJSONBody(t, recorder)
	if int(result["code"].(float64)) != ErrCodeFileTooLarge {
		t.Fatalf("expected code %d, got %v", ErrCodeFileTooLarge, result["code"])
	}
}

func TestPreflightUpload_RejectsDocxOverFivePages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenManager := security.NewTokenManager("test-secret", 180, 180, nil)
	handler := newTestFileHandler(t, tokenManager)
	token, _, _ := tokenManager.GenerateUploadToken("node-1", "printer-1")

	docxContent := buildDocxWithPages(t, 6)
	req, err := createMultipartRequest("/api/v1/files/preflight?token="+token, "too-many-pages.docx", docxContent)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	handler.PreflightUpload(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	result := parseJSONBody(t, recorder)
	if int(result["code"].(float64)) != ErrCodeFileTooManyPages {
		t.Fatalf("expected code %d, got %v", ErrCodeFileTooManyPages, result["code"])
	}
}
