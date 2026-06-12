package business

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"fly-print-cloud/api/internal/config"
)

const (
	KeyUploadMaxSizeBytes      = "upload.max_size_bytes"
	KeyMaxDocumentPages        = "upload.max_document_pages"
	KeyUploadTokenTTLSeconds   = "security.upload_token_ttl_seconds"
	KeyDownloadTokenTTLSeconds = "security.download_token_ttl_seconds"
	KeyAllowedExtensions       = "upload.allowed_extensions"
)

var (
	DefaultAllowedUploadExtensions = []string{".pdf", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff"}
	settingsKeys                   = []string{KeyUploadMaxSizeBytes, KeyMaxDocumentPages, KeyUploadTokenTTLSeconds, KeyDownloadTokenTTLSeconds, KeyAllowedExtensions}
	extensionPattern               = regexp.MustCompile(`^\.[a-z0-9]+$`)
)

type Store interface {
	GetValues(keys []string) (map[string]string, error)
	SetValues(values map[string]string) error
}

type Settings struct {
	UploadMaxSizeBytes      int64    `json:"upload_max_size_bytes"`
	MaxDocumentPages        int      `json:"max_document_pages"`
	UploadTokenTTLSeconds   int      `json:"upload_token_ttl_seconds"`
	DownloadTokenTTLSeconds int      `json:"download_token_ttl_seconds"`
	AllowedExtensions       []string `json:"allowed_extensions"`
}

type SettingsService struct {
	store    Store
	defaults Settings
}

func NewSettingsService(store Store, cfg *config.Config) *SettingsService {
	return &SettingsService{
		store:    store,
		defaults: defaultsFromConfig(cfg),
	}
}

func (s *SettingsService) Current() (Settings, error) {
	settings := s.defaults
	if s == nil || s.store == nil {
		return settings, nil
	}

	values, err := s.store.GetValues(settingsKeys)
	if err != nil {
		return settings, nil
	}

	settings.UploadMaxSizeBytes = parseInt64(values[KeyUploadMaxSizeBytes], settings.UploadMaxSizeBytes)
	settings.MaxDocumentPages = parseInt(values[KeyMaxDocumentPages], settings.MaxDocumentPages)
	settings.UploadTokenTTLSeconds = parseInt(values[KeyUploadTokenTTLSeconds], settings.UploadTokenTTLSeconds)
	settings.DownloadTokenTTLSeconds = parseInt(values[KeyDownloadTokenTTLSeconds], settings.DownloadTokenTTLSeconds)
	if extensions := parseExtensions(values[KeyAllowedExtensions]); len(extensions) > 0 {
		settings.AllowedExtensions = extensions
	}

	if err := validateSettings(settings); err != nil {
		return s.defaults, nil
	}
	return settings, nil
}

func (s *SettingsService) Update(settings Settings) (Settings, error) {
	normalized, err := normalizeSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	if s == nil || s.store == nil {
		return normalized, nil
	}

	err = s.store.SetValues(map[string]string{
		KeyUploadMaxSizeBytes:      strconv.FormatInt(normalized.UploadMaxSizeBytes, 10),
		KeyMaxDocumentPages:        strconv.Itoa(normalized.MaxDocumentPages),
		KeyUploadTokenTTLSeconds:   strconv.Itoa(normalized.UploadTokenTTLSeconds),
		KeyDownloadTokenTTLSeconds: strconv.Itoa(normalized.DownloadTokenTTLSeconds),
		KeyAllowedExtensions:       strings.Join(normalized.AllowedExtensions, ","),
	})
	if err != nil {
		return Settings{}, err
	}
	return normalized, nil
}

func defaultsFromConfig(cfg *config.Config) Settings {
	settings := Settings{
		UploadMaxSizeBytes:      10 * 1024 * 1024,
		MaxDocumentPages:        5,
		UploadTokenTTLSeconds:   180,
		DownloadTokenTTLSeconds: 180,
		AllowedExtensions:       append([]string{}, DefaultAllowedUploadExtensions...),
	}
	if cfg == nil {
		return settings
	}
	if cfg.Storage.MaxSize > 0 {
		settings.UploadMaxSizeBytes = cfg.Storage.MaxSize
	}
	if cfg.Storage.MaxDocumentPages > 0 {
		settings.MaxDocumentPages = cfg.Storage.MaxDocumentPages
	}
	if cfg.Security.UploadTokenTTL > 0 {
		settings.UploadTokenTTLSeconds = cfg.Security.UploadTokenTTL
	}
	if cfg.Security.DownloadTokenTTL > 0 {
		settings.DownloadTokenTTLSeconds = cfg.Security.DownloadTokenTTL
	}
	return settings
}

func normalizeSettings(settings Settings) (Settings, error) {
	normalized := settings
	extensions, err := normalizeExtensions(settings.AllowedExtensions)
	if err != nil {
		return Settings{}, err
	}
	normalized.AllowedExtensions = extensions
	if err := validateSettings(normalized); err != nil {
		return Settings{}, err
	}
	return normalized, nil
}

func validateSettings(settings Settings) error {
	if settings.UploadMaxSizeBytes <= 0 {
		return fmt.Errorf("upload_max_size_bytes must be greater than 0")
	}
	if settings.MaxDocumentPages <= 0 {
		return fmt.Errorf("max_document_pages must be greater than 0")
	}
	if settings.UploadTokenTTLSeconds <= 0 {
		return fmt.Errorf("upload_token_ttl_seconds must be greater than 0")
	}
	if settings.DownloadTokenTTLSeconds <= 0 {
		return fmt.Errorf("download_token_ttl_seconds must be greater than 0")
	}
	if len(settings.AllowedExtensions) == 0 {
		return fmt.Errorf("allowed_extensions must not be empty")
	}
	return nil
}

func normalizeExtensions(values []string) ([]string, error) {
	seen := map[string]bool{}
	extensions := make([]string, 0, len(values))
	for _, value := range values {
		extension := strings.ToLower(strings.TrimSpace(value))
		if extension == "" {
			continue
		}
		if !extensionPattern.MatchString(extension) {
			return nil, fmt.Errorf("allowed extension %q must start with . and contain only letters or digits", value)
		}
		if !seen[extension] {
			seen[extension] = true
			extensions = append(extensions, extension)
		}
	}
	return extensions, nil
}

func parseExtensions(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	extensions, err := normalizeExtensions(strings.Split(value, ","))
	if err != nil {
		return nil
	}
	return extensions
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
