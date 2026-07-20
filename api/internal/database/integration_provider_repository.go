package database

import (
	"database/sql"

	"fly-print-cloud/api/internal/models"
)

// IntegrationProviderRepository persists provider configuration. Secret
// ciphertext is only selected for Cloud-side authentication and callback work.
type IntegrationProviderRepository struct {
	db *DB
}

func NewIntegrationProviderRepository(db *DB) *IntegrationProviderRepository {
	return &IntegrationProviderRepository{db: db}
}

func (r *IntegrationProviderRepository) List() ([]*models.IntegrationProvider, error) {
	rows, err := r.db.Query(providerSelect + ` FROM integration_providers ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]*models.IntegrationProvider, 0)
	for rows.Next() {
		provider := &models.IntegrationProvider{}
		if err := scanProvider(rows, provider, false); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

// Get never exposes secrets to handlers unless withSecrets is explicitly true.
// The latter is reserved for HMAC validation and callback delivery.
func (r *IntegrationProviderRepository) Get(code string, withSecrets bool) (*models.IntegrationProvider, error) {
	query := providerSelect
	if withSecrets {
		query += `,inbound_secret_encrypted,outbound_secret_encrypted`
	}
	query += ` FROM integration_providers WHERE code=$1`

	provider := &models.IntegrationProvider{}
	if err := scanProvider(r.db.QueryRow(query, code), provider, withSecrets); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return provider, nil
}

func (r *IntegrationProviderRepository) Create(provider *models.IntegrationProvider) error {
	return r.db.QueryRow(`INSERT INTO integration_providers(
		code,display_name,entry_url,callback_base_url,entry_visible,enabled,
		allowed_ip_cidrs,allowed_file_hosts,max_file_size,allowed_mime_types,
		inbound_secret_encrypted,outbound_secret_encrypted
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	RETURNING id,created_at,updated_at`,
		provider.Code, provider.DisplayName, provider.EntryURL, provider.CallbackBaseURL,
		provider.EntryVisible, provider.Enabled, provider.AllowedIPCIDRs, provider.AllowedFileHosts,
		provider.MaxFileSize, provider.AllowedMIMETypes, provider.InboundSecretEncrypted,
		provider.OutboundSecretEncrypted,
	).Scan(&provider.ID, &provider.CreatedAt, &provider.UpdatedAt)
}

// Update intentionally excludes both secret columns. Keys can only change via
// RotateSecrets, whose handler returns them exactly once to the administrator.
func (r *IntegrationProviderRepository) Update(provider *models.IntegrationProvider) error {
	return r.db.QueryRow(`UPDATE integration_providers SET
		display_name=$2,entry_url=$3,callback_base_url=$4,entry_visible=$5,enabled=$6,
		allowed_ip_cidrs=$7,allowed_file_hosts=$8,max_file_size=$9,allowed_mime_types=$10
		WHERE code=$1 RETURNING updated_at`,
		provider.Code, provider.DisplayName, provider.EntryURL, provider.CallbackBaseURL,
		provider.EntryVisible, provider.Enabled, provider.AllowedIPCIDRs, provider.AllowedFileHosts,
		provider.MaxFileSize, provider.AllowedMIMETypes,
	).Scan(&provider.UpdatedAt)
}

// UpdateFlags changes the two operational switches without replacing the
// provider's connection and file policy configuration.
func (r *IntegrationProviderRepository) UpdateFlags(code string, entryVisible, enabled bool) (*models.IntegrationProvider, error) {
	provider := &models.IntegrationProvider{}
	err := scanProvider(r.db.QueryRow(`UPDATE integration_providers SET entry_visible=$2,enabled=$3
		WHERE code=$1 RETURNING id,code,display_name,entry_url,callback_base_url,entry_visible,enabled,
		allowed_ip_cidrs,allowed_file_hosts,max_file_size,allowed_mime_types,created_at,updated_at`, code, entryVisible, enabled), provider, false)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return provider, nil
}

func (r *IntegrationProviderRepository) RotateSecrets(code, inbound, outbound string) error {
	_, err := r.db.Exec(`UPDATE integration_providers
		SET inbound_secret_encrypted=$2,outbound_secret_encrypted=$3 WHERE code=$1`, code, inbound, outbound)
	return err
}

const providerSelect = `SELECT id,code,display_name,entry_url,callback_base_url,entry_visible,enabled,
	allowed_ip_cidrs,allowed_file_hosts,max_file_size,allowed_mime_types,created_at,updated_at`

func scanProvider(scanner interface{ Scan(...any) error }, provider *models.IntegrationProvider, withSecrets bool) error {
	fields := []any{
		&provider.ID, &provider.Code, &provider.DisplayName, &provider.EntryURL, &provider.CallbackBaseURL,
		&provider.EntryVisible, &provider.Enabled, &provider.AllowedIPCIDRs, &provider.AllowedFileHosts,
		&provider.MaxFileSize, &provider.AllowedMIMETypes, &provider.CreatedAt, &provider.UpdatedAt,
	}
	if withSecrets {
		fields = append(fields, &provider.InboundSecretEncrypted, &provider.OutboundSecretEncrypted)
	}
	return scanner.Scan(fields...)
}
