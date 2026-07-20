package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed *.sql
var files embed.FS

// Run records every applied schema change. Migrations are forward-only and
// transactional, so API instances can safely start concurrently.
func Run(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := files.ReadDir(".")
	if err != nil { return err }
	names := make([]string, 0, len(entries))
	for _, entry := range entries { if !entry.IsDir() { names = append(names, entry.Name()) } }
	sort.Strings(names)
	for _, name := range names {
		var applied bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&applied); err != nil { return err }
		if applied { continue }
		sqlText, err := files.ReadFile(name); if err != nil { return err }
		tx, err := db.Begin(); if err != nil { return err }
		if _, err = tx.Exec(string(sqlText)); err == nil { _, err = tx.Exec(`INSERT INTO schema_migrations(version) VALUES($1)`, name) }
		if err != nil { _ = tx.Rollback(); return fmt.Errorf("apply migration %s: %w", name, err) }
		if err = tx.Commit(); err != nil { return fmt.Errorf("commit migration %s: %w", name, err) }
	}
	return nil
}
