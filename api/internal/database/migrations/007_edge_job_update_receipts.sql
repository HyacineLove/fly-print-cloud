CREATE TABLE IF NOT EXISTS edge_job_update_receipts (
  event_id UUID PRIMARY KEY,
  node_id VARCHAR(100) NOT NULL REFERENCES edge_nodes(id),
  job_id UUID NOT NULL REFERENCES print_jobs(id),
  status VARCHAR(20) NOT NULL CHECK (status IN ('completed','failed','canceled','unconfirmed')),
  payload_hash CHAR(64) NOT NULL,
  received_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_edge_job_update_receipts_job_id
  ON edge_job_update_receipts(job_id);
