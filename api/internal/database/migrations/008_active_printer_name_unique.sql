ALTER TABLE printers DROP CONSTRAINT IF EXISTS printers_name_edge_node_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_printers_active_name_edge_node_unique
  ON printers(name, edge_node_id)
  WHERE deleted_at IS NULL;
