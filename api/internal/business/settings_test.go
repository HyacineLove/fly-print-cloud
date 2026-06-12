package business

import (
	"errors"
	"reflect"
	"testing"

	"fly-print-cloud/api/internal/config"
)

type memorySettingsStore struct {
	values map[string]string
	err    error
}

func (s *memorySettingsStore) GetValues(keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *memorySettingsStore) SetValues(values map[string]string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range values {
		s.values[key] = value
	}
	return nil
}

func TestSettingsServiceFallsBackToStaticDefaults(t *testing.T) {
	t.Parallel()

	service := NewSettingsService(&memorySettingsStore{}, defaultConfig())

	settings, err := service.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if settings.UploadMaxSizeBytes != 20*1024*1024 {
		t.Fatalf("UploadMaxSizeBytes = %d, want %d", settings.UploadMaxSizeBytes, 20*1024*1024)
	}
	if settings.MaxDocumentPages != 8 {
		t.Fatalf("MaxDocumentPages = %d, want %d", settings.MaxDocumentPages, 8)
	}
	if settings.UploadTokenTTLSeconds != 240 {
		t.Fatalf("UploadTokenTTLSeconds = %d, want %d", settings.UploadTokenTTLSeconds, 240)
	}
	if settings.DownloadTokenTTLSeconds != 360 {
		t.Fatalf("DownloadTokenTTLSeconds = %d, want %d", settings.DownloadTokenTTLSeconds, 360)
	}
	if !reflect.DeepEqual(settings.AllowedExtensions, DefaultAllowedUploadExtensions) {
		t.Fatalf("AllowedExtensions = %#v, want %#v", settings.AllowedExtensions, DefaultAllowedUploadExtensions)
	}
}

func TestSettingsServiceUsesStoredValues(t *testing.T) {
	t.Parallel()

	store := &memorySettingsStore{values: map[string]string{
		KeyUploadMaxSizeBytes:      "1048576",
		KeyMaxDocumentPages:        "3",
		KeyUploadTokenTTLSeconds:   "90",
		KeyDownloadTokenTTLSeconds: "120",
		KeyAllowedExtensions:       ".pdf,.png",
	}}
	service := NewSettingsService(store, defaultConfig())

	settings, err := service.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if settings.UploadMaxSizeBytes != 1048576 {
		t.Fatalf("UploadMaxSizeBytes = %d, want %d", settings.UploadMaxSizeBytes, 1048576)
	}
	if settings.MaxDocumentPages != 3 {
		t.Fatalf("MaxDocumentPages = %d, want %d", settings.MaxDocumentPages, 3)
	}
	if settings.UploadTokenTTLSeconds != 90 {
		t.Fatalf("UploadTokenTTLSeconds = %d, want %d", settings.UploadTokenTTLSeconds, 90)
	}
	if settings.DownloadTokenTTLSeconds != 120 {
		t.Fatalf("DownloadTokenTTLSeconds = %d, want %d", settings.DownloadTokenTTLSeconds, 120)
	}
	if !reflect.DeepEqual(settings.AllowedExtensions, []string{".pdf", ".png"}) {
		t.Fatalf("AllowedExtensions = %#v, want %#v", settings.AllowedExtensions, []string{".pdf", ".png"})
	}
}

func TestSettingsServiceRejectsInvalidUpdate(t *testing.T) {
	t.Parallel()

	service := NewSettingsService(&memorySettingsStore{}, defaultConfig())

	_, err := service.Update(Settings{
		UploadMaxSizeBytes:      0,
		MaxDocumentPages:        3,
		UploadTokenTTLSeconds:   90,
		DownloadTokenTTLSeconds: 120,
		AllowedExtensions:       []string{".pdf"},
	})
	if err == nil {
		t.Fatalf("Update() error = nil, want validation error")
	}

	_, err = service.Update(Settings{
		UploadMaxSizeBytes:      1024,
		MaxDocumentPages:        3,
		UploadTokenTTLSeconds:   90,
		DownloadTokenTTLSeconds: 120,
		AllowedExtensions:       []string{"pdf"},
	})
	if err == nil {
		t.Fatalf("Update() error = nil, want extension validation error")
	}
}

func TestSettingsServiceFallsBackWhenStoreFails(t *testing.T) {
	t.Parallel()

	service := NewSettingsService(&memorySettingsStore{err: errors.New("db down")}, defaultConfig())

	settings, err := service.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if settings.UploadMaxSizeBytes != 20*1024*1024 {
		t.Fatalf("UploadMaxSizeBytes = %d, want fallback", settings.UploadMaxSizeBytes)
	}
}

func defaultConfig() *config.Config {
	return &config.Config{
		Storage: config.StorageConfig{
			MaxSize:          20 * 1024 * 1024,
			MaxDocumentPages: 8,
		},
		Security: config.SecurityConfig{
			UploadTokenTTL:   240,
			DownloadTokenTTL: 360,
		},
	}
}
