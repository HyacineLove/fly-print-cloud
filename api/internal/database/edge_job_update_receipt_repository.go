package database

import (
	"database/sql"
	"fmt"
)

// EdgeJobUpdateReceipt records an accepted terminal result from an Edge node.
// It makes Edge retries idempotent even when the acknowledgement is lost.
type EdgeJobUpdateReceipt struct {
	EventID     string
	NodeID      string
	JobID       string
	Status      string
	PayloadHash string
}

type EdgeJobUpdateReceiptRepository struct{ db *DB }

func NewEdgeJobUpdateReceiptRepository(db *DB) *EdgeJobUpdateReceiptRepository {
	return &EdgeJobUpdateReceiptRepository{db: db}
}

func (r *EdgeJobUpdateReceiptRepository) GetTx(tx *Tx, eventID string) (*EdgeJobUpdateReceipt, error) {
	receipt := &EdgeJobUpdateReceipt{}
	err := tx.QueryRow(`SELECT event_id::text,node_id,job_id::text,status,payload_hash
		FROM edge_job_update_receipts WHERE event_id=$1::uuid`, eventID).
		Scan(&receipt.EventID, &receipt.NodeID, &receipt.JobID, &receipt.Status, &receipt.PayloadHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Edge job-update receipt: %w", err)
	}
	return receipt, nil
}

func (r *EdgeJobUpdateReceiptRepository) CreateTx(tx *Tx, receipt EdgeJobUpdateReceipt) error {
	_, err := tx.Exec(`INSERT INTO edge_job_update_receipts(event_id,node_id,job_id,status,payload_hash)
		VALUES($1::uuid,$2,$3::uuid,$4,$5)`, receipt.EventID, receipt.NodeID, receipt.JobID, receipt.Status, receipt.PayloadHash)
	if err != nil {
		return fmt.Errorf("create Edge job-update receipt: %w", err)
	}
	return nil
}
