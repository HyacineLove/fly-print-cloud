package database

import (
	"database/sql"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"
)

type TerminalTicketRepository struct{ db *DB }

func NewTerminalTicketRepository(db *DB) *TerminalTicketRepository {
	return &TerminalTicketRepository{db: db}
}

func (r *TerminalTicketRepository) Create(ticket *models.TerminalTicket) error {
	return r.db.QueryRow(`INSERT INTO terminal_tickets
		(ticket_hash, node_id, printer_id, terminal_session_id, status, expires_at)
		VALUES ($1,$2,$3,$4,'issued',$5) RETURNING id, issued_at`,
		ticket.TicketHash, ticket.NodeID, ticket.PrinterID, ticket.TerminalSessionID, ticket.ExpiresAt,
	).Scan(&ticket.ID, &ticket.IssuedAt)
}

func (r *TerminalTicketRepository) GetValidByHash(hash string, now time.Time) (*models.TerminalTicket, error) {
	row := r.db.QueryRow(`SELECT id, ticket_hash, node_id, printer_id, terminal_session_id, selected_entry,
		status, issued_at, selected_at, consumed_at, expires_at FROM terminal_tickets
		WHERE ticket_hash=$1 AND status IN ('issued','selected') AND expires_at>$2`, hash, now)
	return scanTerminalTicket(row)
}

// Select atomically locks a ticket to one entry. Replays and switching are
// deliberately conflicts, never silently overwritten.
func (r *TerminalTicketRepository) Select(hash, entry string, now time.Time) (*models.TerminalTicket, error) {
	row := r.db.QueryRow(`UPDATE terminal_tickets SET selected_entry=$2,status='selected',selected_at=$3
		WHERE ticket_hash=$1 AND status='issued' AND expires_at>$3
		RETURNING id,ticket_hash,node_id,printer_id,terminal_session_id,selected_entry,status,issued_at,selected_at,consumed_at,expires_at`, hash, entry, now)
	ticket, err := scanTerminalTicket(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket cannot be selected")
	}
	return ticket, err
}

func scanTerminalTicket(scanner interface{ Scan(...any) error }) (*models.TerminalTicket, error) {
	ticket := &models.TerminalTicket{}
	if err := scanner.Scan(&ticket.ID, &ticket.TicketHash, &ticket.NodeID, &ticket.PrinterID,
		&ticket.TerminalSessionID, &ticket.SelectedEntry, &ticket.Status, &ticket.IssuedAt,
		&ticket.SelectedAt, &ticket.ConsumedAt, &ticket.ExpiresAt); err != nil {
		return nil, err
	}
	return ticket, nil
}
