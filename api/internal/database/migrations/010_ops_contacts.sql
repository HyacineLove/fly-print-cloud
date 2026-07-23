-- Ops contact persons (display-only; not login accounts) and node bindings.
-- edge_nodes.id is VARCHAR(100), not UUID.

CREATE TABLE IF NOT EXISTS ops_contacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL,
  phone VARCHAR(40) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  deleted_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ops_contacts_active
  ON ops_contacts(enabled, deleted_at)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS node_ops_contacts (
  edge_node_id VARCHAR(100) NOT NULL REFERENCES edge_nodes(id) ON DELETE CASCADE,
  contact_id UUID NOT NULL REFERENCES ops_contacts(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (edge_node_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_node_ops_contacts_contact
  ON node_ops_contacts(contact_id);
