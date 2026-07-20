CREATE TABLE IF NOT EXISTS edge_terminal_sessions (
  node_id VARCHAR(100) PRIMARY KEY REFERENCES edge_nodes(id) ON DELETE CASCADE,
  terminal_session_id VARCHAR(128),
  terminal_ticket_hash CHAR(64),
  entry_type VARCHAR(64),
  integration_request_id UUID REFERENCES integration_print_requests(id) ON DELETE SET NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE OR REPLACE FUNCTION flyprint_touch_updated_at() RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_integration_providers_updated_at ON integration_providers;
CREATE TRIGGER trg_integration_providers_updated_at BEFORE UPDATE ON integration_providers
FOR EACH ROW EXECUTE FUNCTION flyprint_touch_updated_at();
DROP TRIGGER IF EXISTS trg_integration_print_requests_updated_at ON integration_print_requests;
CREATE TRIGGER trg_integration_print_requests_updated_at BEFORE UPDATE ON integration_print_requests
FOR EACH ROW EXECUTE FUNCTION flyprint_touch_updated_at();

DO $$ BEGIN
  ALTER TABLE integration_print_requests ADD CONSTRAINT integration_print_requests_status_check
  CHECK (status IN ('accepted','waiting_file','waiting_terminal','dispatched','printing','completed','failed','expired','cancelled'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
