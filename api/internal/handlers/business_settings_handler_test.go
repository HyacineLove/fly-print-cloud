package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fly-print-cloud/api/internal/business"

	"github.com/gin-gonic/gin"
)

func newBusinessSettingsTestRouter(service businessSettingsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewBusinessSettingsHandler(service)
	router := gin.New()
	router.GET("/api/v1/admin/business-settings", handler.Get)
	router.PUT("/api/v1/admin/business-settings", handler.Update)
	return router
}

func performBusinessSettingsRequest(
	router *gin.Engine,
	method string,
	body []byte,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		"/api/v1/admin/business-settings",
		bytes.NewReader(body),
	)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestBusinessSettingsHandlerGet(t *testing.T) {
	t.Parallel()

	service := &stubBusinessSettingsService{
		settings: business.Settings{
			UploadMaxSizeBytes:      1024,
			MaxDocumentPages:        2,
			UploadTokenTTLSeconds:   90,
			DownloadTokenTTLSeconds: 120,
			AllowedExtensions:       []string{".pdf"},
		},
	}
	router := newBusinessSettingsTestRouter(service)
	rec := performBusinessSettingsRequest(router, http.MethodGet, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"upload_max_size_bytes":1024`) {
		t.Fatalf("body = %s, want upload max size", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"allowed_extensions":[".pdf"]`) {
		t.Fatalf("body = %s, want allowed extensions", rec.Body.String())
	}
}

func TestBusinessSettingsHandlerUpdate(t *testing.T) {
	t.Parallel()

	service := &stubBusinessSettingsService{}
	router := newBusinessSettingsTestRouter(service)

	body, _ := json.Marshal(business.Settings{
		UploadMaxSizeBytes:      2048,
		MaxDocumentPages:        4,
		UploadTokenTTLSeconds:   180,
		DownloadTokenTTLSeconds: 240,
		AllowedExtensions:       []string{".pdf", ".png"},
	})
	rec := performBusinessSettingsRequest(router, http.MethodPut, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.updated.UploadMaxSizeBytes != 2048 {
		t.Fatalf("updated UploadMaxSizeBytes = %d, want %d", service.updated.UploadMaxSizeBytes, 2048)
	}
}

func TestBusinessSettingsHandlerUpdateValidationError(t *testing.T) {
	t.Parallel()

	service := &stubBusinessSettingsService{updateErr: errors.New("upload_max_size_bytes must be greater than 0")}
	router := newBusinessSettingsTestRouter(service)

	body := []byte(`{"upload_max_size_bytes":0,"max_document_pages":4,"upload_token_ttl_seconds":180,"download_token_ttl_seconds":240,"allowed_extensions":[".pdf"]}`)
	rec := performBusinessSettingsRequest(router, http.MethodPut, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "upload_max_size_bytes must be greater than 0") {
		t.Fatalf("body = %s, want validation error", rec.Body.String())
	}
}

type stubBusinessSettingsService struct {
	settings  business.Settings
	updated   business.Settings
	err       error
	updateErr error
}

func (s *stubBusinessSettingsService) Current() (business.Settings, error) {
	return s.settings, s.err
}

func (s *stubBusinessSettingsService) Update(settings business.Settings) (business.Settings, error) {
	if s.updateErr != nil {
		return business.Settings{}, s.updateErr
	}
	s.updated = settings
	s.settings = settings
	return settings, nil
}
