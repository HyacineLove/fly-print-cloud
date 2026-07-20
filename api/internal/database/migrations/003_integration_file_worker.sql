ALTER TABLE integration_print_requests
  ADD COLUMN IF NOT EXISTS worker_lease_until TIMESTAMP;
CREATE INDEX IF NOT EXISTS idx_integration_requests_file_lease
  ON integration_print_requests(status, worker_lease_until, created_at)
  WHERE status = 'waiting_file';
