ALTER TABLE integration_print_requests
  ADD COLUMN IF NOT EXISTS file_expires_at TIMESTAMP;

UPDATE integration_print_requests
SET file_expires_at = expires_at
WHERE file_expires_at IS NULL;

ALTER TABLE integration_print_requests
  ALTER COLUMN file_expires_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_integration_requests_file_expiry
  ON integration_print_requests(status, file_expires_at);
