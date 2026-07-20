package database

import "time"

// TerminalSessionRepository is the Cloud-side source of truth for the active
// kiosk session. An Edge restart reports an empty session, so queued work is
// never dispatched based only on stale ticket data.
type TerminalSessionRepository struct{ db *DB }

func NewTerminalSessionRepository(db *DB) *TerminalSessionRepository { return &TerminalSessionRepository{db: db} }

func (r *TerminalSessionRepository) Report(nodeID, sessionID, ticketHash, entryType, integrationRequestID string, now time.Time) error {
	_, err := r.db.Exec(`INSERT INTO edge_terminal_sessions(node_id,terminal_session_id,terminal_ticket_hash,entry_type,integration_request_id,updated_at)
		VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,'')::uuid,$6)
		ON CONFLICT(node_id) DO UPDATE SET terminal_session_id=EXCLUDED.terminal_session_id,terminal_ticket_hash=EXCLUDED.terminal_ticket_hash,
		entry_type=EXCLUDED.entry_type,integration_request_id=EXCLUDED.integration_request_id,updated_at=EXCLUDED.updated_at`,
		nodeID, sessionID, ticketHash, entryType, integrationRequestID, now)
	return err
}

func (r *TerminalSessionRepository) Matches(nodeID, sessionID, ticketHash, integrationRequestID string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM edge_terminal_sessions WHERE node_id=$1 AND terminal_session_id=$2 AND terminal_ticket_hash=$3
		AND (integration_request_id IS NOT DISTINCT FROM NULLIF($4,'')::uuid)`, nodeID, sessionID, ticketHash, integrationRequestID).Scan(&count)
	return count == 1, err
}
