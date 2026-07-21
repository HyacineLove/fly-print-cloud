package models

import "time"

// IntegrationProvider contains only non-secret provider configuration.
type IntegrationProvider struct {
	ID                      string    `json:"id"`
	Code                    string    `json:"code"`
	DisplayName             string    `json:"display_name"`
	EntryURL                string    `json:"entry_url"`
	CallbackBaseURL         string    `json:"callback_base_url"`
	EntryVisible            bool      `json:"entry_visible"`
	Enabled                 bool      `json:"enabled"`
	AllowedIPCIDRs          string    `json:"allowed_ip_cidrs"`
	AllowedFileHosts        string    `json:"allowed_file_hosts"`
	AllowPrivateFileHosts   bool      `json:"allow_private_file_hosts"`
	MaxFileSize             int64     `json:"max_file_size"`
	AllowedMIMETypes        string    `json:"allowed_mime_types"`
	InboundSecretEncrypted  string    `json:"-"`
	OutboundSecretEncrypted string    `json:"-"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
