ALTER TABLE integration_providers
  ADD COLUMN IF NOT EXISTS allow_private_file_hosts BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE integration_print_requests
  ADD COLUMN IF NOT EXISTS preview_sent_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_integration_requests_preview_due
  ON integration_print_requests(status, preview_sent_at, expires_at)
  WHERE status = 'waiting_terminal';
