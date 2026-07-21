package models

import "time"

// IntegrationPrintRequest is the integration-domain record. It keeps provider
// state outside print_jobs and is linked to an internal job only after Cloud
// has accepted the provider file into its own storage.
type IntegrationPrintRequest struct {
	ID                 string     `json:"id"`
	ProviderCode       string     `json:"provider_code"`
	ExternalOrderID    string     `json:"external_order_id"`
	RequestHash        string     `json:"-"`
	TerminalTicketHash string     `json:"-"`
	NodeID             string     `json:"node_id"`
	PrinterID          string     `json:"printer_id"`
	TerminalSessionID  string     `json:"terminal_session_id"`
	ExternalUserID     string     `json:"external_user_id"`
	ExternalUserName   string     `json:"external_user_name"`
	FileURL            string     `json:"-"`
	FileName           string     `json:"file_name"`
	FileSize           int64      `json:"file_size"`
	MimeType           string     `json:"mime_type"`
	FileSHA256         string     `json:"file_sha256"`
	FileExpiresAt      time.Time  `json:"file_expires_at"`
	PrintOptions       []byte     `json:"print_options"`
	Metadata           []byte     `json:"metadata"`
	FileID             *string    `json:"file_id,omitempty"`
	PrintJobID         *string    `json:"flyprint_job_id,omitempty"`
	PreviewSentAt      *time.Time `json:"preview_sent_at,omitempty"`
	Status             string     `json:"status"`
	ErrorCode          *string    `json:"error_code,omitempty"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	ExpiresAt          time.Time  `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
