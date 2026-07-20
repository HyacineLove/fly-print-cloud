ALTER TABLE edge_nodes
  ADD COLUMN IF NOT EXISTS alias VARCHAR(100),
  ADD COLUMN IF NOT EXISTS registration_state VARCHAR(32) NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS activation_code_hash CHAR(64),
  ADD COLUMN IF NOT EXISTS activation_expires_at TIMESTAMP,
  ADD COLUMN IF NOT EXISTS activated_at TIMESTAMP;

UPDATE edge_nodes SET registration_state='active' WHERE registration_state IS NULL;

ALTER TABLE edge_nodes DROP CONSTRAINT IF EXISTS edge_nodes_registration_state_check;
ALTER TABLE edge_nodes ADD CONSTRAINT edge_nodes_registration_state_check
  CHECK (registration_state IN ('pending_activation','registered','active','disabled'));

CREATE INDEX IF NOT EXISTS idx_edge_nodes_pending_activation
  ON edge_nodes(registration_state, activation_expires_at)
  WHERE deleted_at IS NULL;
