ALTER TABLE print_jobs ALTER COLUMN max_retries SET DEFAULT 3;
UPDATE print_jobs SET max_retries=3 WHERE max_retries IS DISTINCT FROM 3;
