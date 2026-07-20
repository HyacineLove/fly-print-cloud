ALTER TABLE oauth2_clients ADD COLUMN IF NOT EXISTS edge_node_id VARCHAR(100) REFERENCES edge_nodes(id) ON DELETE RESTRICT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth2_clients_edge_node_id ON oauth2_clients(edge_node_id) WHERE edge_node_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS terminal_tickets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), ticket_hash CHAR(64) UNIQUE NOT NULL,
  node_id VARCHAR(100) NOT NULL REFERENCES edge_nodes(id) ON DELETE RESTRICT,
  printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
  terminal_session_id VARCHAR(128) NOT NULL, selected_entry VARCHAR(64),
  status VARCHAR(16) NOT NULL CHECK (status IN ('issued','selected','consumed','expired','cancelled')),
  issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, selected_at TIMESTAMP, consumed_at TIMESTAMP,
  expires_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_terminal_tickets_expires_at ON terminal_tickets(expires_at);

CREATE TABLE IF NOT EXISTS integration_providers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), code VARCHAR(64) UNIQUE NOT NULL,
  display_name VARCHAR(128) NOT NULL, entry_url TEXT NOT NULL, callback_base_url TEXT NOT NULL,
  entry_visible BOOLEAN NOT NULL DEFAULT true, enabled BOOLEAN NOT NULL DEFAULT false,
  allowed_ip_cidrs TEXT NOT NULL, allowed_file_hosts TEXT NOT NULL,
  max_file_size BIGINT NOT NULL, allowed_mime_types TEXT NOT NULL,
  inbound_secret_encrypted TEXT NOT NULL, outbound_secret_encrypted TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS terminal_upload_sessions (
  upload_token_hash CHAR(64) PRIMARY KEY, terminal_ticket_hash CHAR(64) NOT NULL REFERENCES terminal_tickets(ticket_hash),
  node_id VARCHAR(100) NOT NULL, printer_id UUID NOT NULL, terminal_session_id VARCHAR(128) NOT NULL,
  file_id UUID REFERENCES files(id), created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS integration_print_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), provider_code VARCHAR(64) NOT NULL REFERENCES integration_providers(code),
  external_order_id VARCHAR(255) NOT NULL, request_hash CHAR(64) NOT NULL, terminal_ticket_hash CHAR(64) NOT NULL REFERENCES terminal_tickets(ticket_hash),
  node_id VARCHAR(100) NOT NULL, printer_id UUID NOT NULL, terminal_session_id VARCHAR(128) NOT NULL,
  external_user_id VARCHAR(255), external_user_name VARCHAR(255), file_url TEXT NOT NULL, file_name VARCHAR(512) NOT NULL,
  file_size BIGINT NOT NULL, mime_type VARCHAR(255) NOT NULL, file_sha256 CHAR(64) NOT NULL, print_options JSONB NOT NULL DEFAULT '{}'::jsonb,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb, file_id UUID REFERENCES files(id), print_job_id UUID REFERENCES print_jobs(id),
  status VARCHAR(32) NOT NULL, error_code VARCHAR(100), error_message TEXT, expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(provider_code, external_order_id)
);
CREATE INDEX IF NOT EXISTS idx_integration_requests_status ON integration_print_requests(status, created_at);
CREATE TABLE IF NOT EXISTS integration_callback_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), event_id UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  request_id UUID NOT NULL REFERENCES integration_print_requests(id), provider_code VARCHAR(64) NOT NULL REFERENCES integration_providers(code),
  target_url TEXT NOT NULL, payload JSONB NOT NULL, payload_hash CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempt_count INT NOT NULL DEFAULT 0, next_attempt_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_error TEXT, delivered_at TIMESTAMP, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_integration_callback_due ON integration_callback_events(status, next_attempt_at);
