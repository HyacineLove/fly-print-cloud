package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

type TerminalUploadSessionRepository struct{ db *DB }

func NewTerminalUploadSessionRepository(db *DB) *TerminalUploadSessionRepository { return &TerminalUploadSessionRepository{db: db} }

func (r *TerminalUploadSessionRepository) Create(rawToken, ticketHash, nodeID, printerID, sessionID string, expiresAt time.Time) error {
	_, err := r.db.Exec(`INSERT INTO terminal_upload_sessions(upload_token_hash,terminal_ticket_hash,node_id,printer_id,terminal_session_id,expires_at)
		VALUES($1,$2,$3,$4,$5,$6)`, uploadTokenHash(rawToken), ticketHash, nodeID, printerID, sessionID, expiresAt)
	return err
}

// BindFile atomically consumes the official ticket only when upload succeeds.
func (r *TerminalUploadSessionRepository) BindFile(rawToken, fileID string, now time.Time) error {
	tx, err := r.db.Begin(); if err != nil { return err }; defer tx.Rollback()
	var ticketHash string
	err = tx.QueryRow(`SELECT terminal_ticket_hash FROM terminal_upload_sessions WHERE upload_token_hash=$1 AND file_id IS NULL AND expires_at>$2 FOR UPDATE`, uploadTokenHash(rawToken), now).Scan(&ticketHash)
	// Existing Edge QR uploads predate the entry bridge and deliberately have no
	// mapping. Preserve that protocol; a mapping, once present, is strict.
	if err == sql.ErrNoRows { return nil }
	if err != nil { return fmt.Errorf("terminal upload session unavailable: %w", err) }
	result, err := tx.Exec(`UPDATE terminal_tickets SET status='consumed',consumed_at=$2 WHERE ticket_hash=$1 AND status='selected' AND selected_entry='official' AND expires_at>$2`, ticketHash, now)
	if err != nil { return err }
	changed, _ := result.RowsAffected(); if changed != 1 { return fmt.Errorf("terminal ticket unavailable") }
	if _, err := tx.Exec(`UPDATE terminal_upload_sessions SET file_id=$2::uuid WHERE upload_token_hash=$1`, uploadTokenHash(rawToken), fileID); err != nil { return err }
	return tx.Commit()
}

func uploadTokenHash(raw string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) }
