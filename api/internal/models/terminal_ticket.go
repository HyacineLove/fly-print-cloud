package models

import "time"

// TerminalTicket is an opaque, short-lived proof that a user is physically at
// one Edge kiosk. The raw token is returned once and is never stored.
type TerminalTicket struct {
	ID                string     `json:"id"`
	TicketHash        string     `json:"-"`
	NodeID            string     `json:"-"`
	PrinterID         string     `json:"-"`
	TerminalSessionID string     `json:"-"`
	SelectedEntry     *string    `json:"selected_entry,omitempty"`
	Status            string     `json:"status"`
	IssuedAt          time.Time  `json:"issued_at"`
	SelectedAt        *time.Time `json:"selected_at,omitempty"`
	ConsumedAt        *time.Time `json:"consumed_at,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
}
