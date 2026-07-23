package models

import "time"

// OpsContact is a display-only ops person profile (name + phone), not a login account.
type OpsContact struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Phone     string     `json:"phone"`
	Enabled   bool       `json:"enabled"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// NodeIDs is populated on admin get/list when bindings are requested.
	NodeIDs []string `json:"node_ids,omitempty"`
}

// OpsContactPublic is the Edge-facing payload (no ids or admin fields).
type OpsContactPublic struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}
