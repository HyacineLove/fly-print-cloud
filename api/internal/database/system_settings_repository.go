package database

import (
	"fmt"

	"github.com/lib/pq"
)

type SystemSettingsRepository struct {
	db *DB
}

func NewSystemSettingsRepository(db *DB) *SystemSettingsRepository {
	return &SystemSettingsRepository{db: db}
}

func (r *SystemSettingsRepository) GetValues(keys []string) (map[string]string, error) {
	values := map[string]string{}
	if len(keys) == 0 {
		return values, nil
	}

	rows, err := r.db.Query(`
		SELECT key, value
		FROM system_settings
		WHERE key = ANY($1)`, pq.Array(keys))
	if err != nil {
		return nil, fmt.Errorf("failed to query system settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan system setting: %w", err)
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("system settings rows error: %w", err)
	}
	return values, nil
}

func (r *SystemSettingsRepository) SetValues(values map[string]string) error {
	if len(values) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin system settings transaction: %w", err)
	}
	defer tx.Rollback()

	for key, value := range values {
		_, err := tx.Exec(`
			INSERT INTO system_settings (key, value)
			VALUES ($1, $2)
			ON CONFLICT (key) DO UPDATE SET
				value = EXCLUDED.value,
				updated_at = CURRENT_TIMESTAMP`, key, value)
		if err != nil {
			return fmt.Errorf("failed to upsert system setting %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit system settings transaction: %w", err)
	}
	return nil
}
