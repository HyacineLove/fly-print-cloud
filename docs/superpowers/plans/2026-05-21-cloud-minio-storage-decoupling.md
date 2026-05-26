# Cloud MinIO Storage Decoupling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple cloud file storage from the local filesystem, introduce a pluggable storage backend with MinIO support, and preserve the existing QR upload -> preview -> `submit_print_params` -> Edge download flow during the first rollout.

**Architecture:** Add a `StorageService` abstraction to the cloud API, implement `local` and `minio` backends, migrate the `files` table to track object-storage identity, keep download serving in proxy mode for phase 1, and add MinIO services plus bootstrap wiring to `docker-compose`.

**Tech Stack:** Go, Gin, PostgreSQL, MinIO (S3-compatible), Docker Compose, existing Edge Python client for compatibility verification, Go tests, compose config verification.

---

## File Structure

### Storage abstraction and backends

- Create: `api/internal/storage/storage.go`
  - Defines backend interface and shared metadata types.
- Create: `api/internal/storage/local.go`
  - Implements the current local-disk behavior behind the new abstraction.
- Create: `api/internal/storage/minio.go`
  - Implements MinIO object upload, download, delete, stat, and presigned GET support.
- Create: `api/internal/storage/factory.go`
  - Builds the configured backend from `config.Storage`.

### Config and runtime wiring

- Modify: `api/internal/config/config.go`
  - Add storage provider, download mode, and MinIO config fields plus validation.
- Modify: `api/config.example.yaml`
  - Document new storage configuration sections.
- Modify: `api/cmd/server/main.go`
  - Build the storage backend, inject it into handlers and cleanup jobs.

### Data model and repositories

- Modify: `api/internal/models/file.go`
  - Add object-storage identity fields and keep the outward API stable.
- Modify: `api/internal/database/database.go`
  - Add schema migration for storage metadata fields.
- Modify: `api/internal/database/file_repository.go`
  - Read and write the new fields.

### Handler and transport integration

- Modify: `api/internal/handlers/file_handler.go`
  - Replace direct filesystem writes and reads with storage backend calls.
- Modify: `api/internal/websocket/connection.go`
  - Build file URLs from metadata without assuming local-file semantics.
- Modify: `api/internal/websocket/manager.go`
  - Preserve phase-1 `/api/v1/files/{id}` behavior and prepare for later presigned mode.

### Compose and migration tooling

- Modify: `docker-compose.yml`
  - Add MinIO and bucket bootstrap support.
- Modify: `.env.example`
  - Add MinIO-related environment variables and storage provider selection.
- Create: `api/cmd/migrate-files/main.go`
  - Backfill local files into MinIO and update file metadata.

### Tests

- Create: `api/internal/storage/local_test.go`
  - Covers local backend behavior.
- Create: `api/internal/storage/minio_test.go`
  - Covers MinIO backend behavior when test credentials are available.
- Modify: `api/internal/handlers/file_handler_test.go`
  - Add upload/download tests through the storage abstraction.
- Create: `api/internal/database/file_repository_test.go`
  - Covers new metadata fields and migration behavior.

## Task 1: Introduce the storage abstraction without behavior change

**Files:**

- Create: `api/internal/storage/storage.go`
- Create: `api/internal/storage/local.go`
- Create: `api/internal/storage/local_test.go`
- Modify: `api/internal/config/config.go`

- [ ] **Step 1: Write failing tests for the local storage backend**

Cover:

- `Put` stores a file under the configured local root
- `Get` reads the stored file back with metadata
- `Delete` removes the stored file

Run: `go test -mod=mod ./internal/storage -run TestLocal -count=1`

Expected: FAIL because the storage package does not exist yet.

- [ ] **Step 2: Add storage config fields and validation**

Implement:

- `storage.provider`
- `storage.download_mode`
- nested MinIO config fields

Validation rules:

- `provider` must be `local` or `minio`
- `download_mode` must be `proxy` or `presigned`
- MinIO config is required only when `provider=minio`

- [ ] **Step 3: Implement the storage interface and local backend**

Required interface:

- `Put`
- `Get`
- `Delete`
- `Stat`
- `GeneratePresignedGet`

For the local backend:

- `GeneratePresignedGet` can return `ErrNotSupported`
- preserve current file layout compatibility under the configured local root

- [ ] **Step 4: Run tests to verify the abstraction passes locally**

Run: `go test -mod=mod ./internal/storage ./internal/config -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the storage foundation**

```bash
git add api/internal/storage api/internal/config/config.go
git commit -m "feat: add pluggable storage abstraction"
```

## Task 2: Add MinIO backend and runtime wiring

**Files:**

- Create: `api/internal/storage/minio.go`
- Create: `api/internal/storage/minio_test.go`
- Create: `api/internal/storage/factory.go`
- Modify: `api/cmd/server/main.go`
- Modify: `api/config.example.yaml`

- [ ] **Step 1: Write failing MinIO backend tests**

Cover:

- object upload
- object stat
- object download
- object delete

Keep these tests skippable when MinIO test env vars are not present.

Run: `go test -mod=mod ./internal/storage -run TestMinIO -count=1`

Expected: FAIL because the MinIO backend is not implemented yet.

- [ ] **Step 2: Implement the MinIO backend**

Use the official MinIO Go SDK and support:

- endpoint
- SSL on/off
- bucket
- object prefix

Object keys should be deterministic and not depend on local absolute paths.

- [ ] **Step 3: Add a storage factory and wire it in `main.go`**

Requirements:

- build the configured backend once at startup
- inject it into `FileHandler`
- inject it into file-cleanup startup

- [ ] **Step 4: Run targeted tests**

Run: `go test -mod=mod ./internal/storage ./cmd/server -count=1`

Expected: PASS, with MinIO tests skipped unless credentials are configured.

- [ ] **Step 5: Commit MinIO backend wiring**

```bash
git add api/internal/storage api/cmd/server/main.go api/config.example.yaml
git commit -m "feat: add MinIO storage backend"
```

## Task 3: Move file upload and download to the storage layer

**Files:**

- Modify: `api/internal/handlers/file_handler.go`
- Modify: `api/internal/handlers/file_handler_test.go`

- [ ] **Step 1: Write failing handler tests through the storage abstraction**

Cover:

- upload stores content through the backend instead of `SaveUploadedFile`
- download streams content from backend instead of `c.FileAttachment`
- upload failure does not leave partial metadata behind

Run: `go test -mod=mod ./internal/handlers -run TestFileHandlerStorage -count=1`

Expected: FAIL because the handler still writes directly to local disk.

- [ ] **Step 2: Replace direct upload writes with backend `Put`**

Requirements:

- keep current validation order
- keep current token validation order
- continue generating safe server-side filenames
- no direct dependency on `os.MkdirAll` or `c.SaveUploadedFile` in the upload path

- [ ] **Step 3: Replace direct download reads with backend `Get`**

Requirements:

- keep current auth and token checks
- set `Content-Disposition` using the original filename
- preserve MIME type when available

- [ ] **Step 4: Run handler tests**

Run: `go test -mod=mod ./internal/handlers -count=1`

Expected: PASS.

- [ ] **Step 5: Commit handler storage decoupling**

```bash
git add api/internal/handlers/file_handler.go api/internal/handlers/file_handler_test.go
git commit -m "refactor: route file upload and download through storage service"
```

## Task 4: Migrate file metadata schema and repository behavior

**Files:**

- Modify: `api/internal/models/file.go`
- Modify: `api/internal/database/database.go`
- Modify: `api/internal/database/file_repository.go`
- Create: `api/internal/database/file_repository_test.go`

- [ ] **Step 1: Write failing repository tests for object metadata**

Cover:

- create and read `storage_provider`
- create and read `storage_bucket`
- create and read `object_key`
- backward-compatible read of old rows without these fields populated

Run: `go test -mod=mod ./internal/database -run TestFileRepository -count=1`

Expected: FAIL until schema and repository changes are implemented.

- [ ] **Step 2: Add backward-compatible schema migration**

Requirements:

- do not drop `file_path`
- add new fields with safe defaults
- existing rows remain readable

- [ ] **Step 3: Update model and repository code**

Rules:

- `object_key` becomes the canonical storage locator
- `file_path` remains as migration compatibility only
- new writes populate provider, bucket, and object key

- [ ] **Step 4: Run database tests**

Run: `go test -mod=mod ./internal/database -count=1`

Expected: PASS.

- [ ] **Step 5: Commit metadata migration support**

```bash
git add api/internal/models/file.go api/internal/database/database.go api/internal/database/file_repository.go api/internal/database/file_repository_test.go
git commit -m "feat: track object-storage metadata for files"
```

## Task 5: Keep WebSocket and Edge behavior stable in phase 1

**Files:**

- Modify: `api/internal/websocket/connection.go`
- Modify: `api/internal/websocket/manager.go`

- [ ] **Step 1: Audit and update code that assumes local-path semantics**

Focus areas:

- file URL generation for preview dispatch
- file URL generation for print dispatch
- token generation based on `file_id`

The phase-1 goal is to keep:

- `file_url = /api/v1/files/{id}`
- download token generation behavior unchanged

- [ ] **Step 2: Add guardrails for later presigned mode without enabling it by default**

Requirements:

- keep proxy mode as the default
- make later presigned-mode wiring explicit and config-gated

- [ ] **Step 3: Run targeted tests**

Run: `go test -mod=mod ./internal/websocket -count=1`

Expected: PASS.

- [ ] **Step 4: Commit transport compatibility updates**

```bash
git add api/internal/websocket/connection.go api/internal/websocket/manager.go
git commit -m "refactor: preserve Edge file transport compatibility for MinIO rollout"
```

## Task 6: Refactor cleanup and add backfill tooling

**Files:**

- Modify: `api/cmd/server/main.go`
- Create: `api/cmd/migrate-files/main.go`

- [ ] **Step 1: Replace cleanup filesystem deletes with storage deletes**

Requirements:

- cleanup must delete through the correct backend based on file metadata
- cleanup must tolerate already-missing local files or objects

- [ ] **Step 2: Implement a backfill CLI**

The migration tool should:

- scan DB for `local` records
- read local files
- upload to MinIO
- update metadata
- continue on error and print a summary

The tool must be idempotent.

- [ ] **Step 3: Verify migration tool builds**

Run:

```bash
go test -mod=mod ./cmd/server ./cmd/migrate-files -count=1
go build ./cmd/migrate-files
```

Expected: PASS.

- [ ] **Step 4: Commit cleanup and migration tooling**

```bash
git add api/cmd/server/main.go api/cmd/migrate-files
git commit -m "feat: add MinIO cleanup and file backfill tooling"
```

## Task 7: Add Docker Compose MinIO support and environment wiring

**Files:**

- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] **Step 1: Add MinIO services to compose**

Add:

- `minio`
- optional `minio-init` or `mc` bootstrap service

Requirements:

- API can still boot with `provider=local`
- MinIO stack can boot with `provider=minio`

- [ ] **Step 2: Add environment variables to `.env.example`**

Include:

- `STORAGE_PROVIDER`
- `STORAGE_DOWNLOAD_MODE`
- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_BUCKET`
- `MINIO_USE_SSL`
- `MINIO_OBJECT_PREFIX`

- [ ] **Step 3: Verify compose rendering**

Run: `docker compose config`

Expected: PASS with MinIO and API env vars expanded correctly.

- [ ] **Step 4: Optionally smoke-test MinIO-only infrastructure**

Run: `docker compose up -d postgres minio api`

Expected: containers reach healthy/running state.

- [ ] **Step 5: Commit compose support**

```bash
git add docker-compose.yml .env.example
git commit -m "feat: add MinIO services to local compose stack"
```

## Task 8: Final verification and rollout checks

**Files:**

- Verify: `api/internal/storage/*`, `api/internal/handlers/file_handler.go`, `api/internal/database/*`, `api/internal/websocket/*`, `api/cmd/server/main.go`, `docker-compose.yml`
- Verify manually against `C:\Users\ShiroNeko\Desktop\FlyPrint\fly-print-edge`

- [ ] **Step 1: Run targeted Go verification**

Run:

```bash
go test -mod=mod ./...
```

Expected: PASS.

- [ ] **Step 2: Run compose verification**

Run:

```bash
docker compose config
docker compose up -d --build
```

Expected: PASS.

- [ ] **Step 3: Execute manual end-to-end verification**

Manual checks:

1. Request upload token from Edge and open the QR upload page.
2. Upload a supported file and confirm cloud upload succeeds.
3. Confirm Edge preview still renders through the existing preview page.
4. Submit print params and confirm Edge downloads the file and prints.
5. Restart the Edge client during a pending job and confirm redispatch still works.
6. Switch between `provider=local` and `provider=minio` and verify both paths.
7. Run the backfill tool on a local-backed dataset and confirm migrated files still download.

- [ ] **Step 4: Record any spec drift**

If implementation forces a design correction, update:

- `docs/superpowers/specs/2026-05-21-cloud-minio-storage-decoupling-design.md`

- [ ] **Step 5: Commit final verification fixes**

```bash
git add api docker-compose.yml .env.example docs/superpowers/specs/2026-05-21-cloud-minio-storage-decoupling-design.md
git commit -m "test: verify MinIO storage decoupling rollout"
```

## Recommended Rollout Order

Use this branch order unless runtime findings force a pause:

1. storage abstraction
2. MinIO backend
3. handler refactor
4. schema migration
5. cleanup and backfill
6. compose integration
7. end-to-end verification

## Notes for Execution

- Keep Edge unchanged in phase 1 unless verification proves a compatibility gap.
- Do not enable `presigned` mode by default during the first rollout.
- Prefer dual-read compatibility during migration over a hard cutover.
- Treat the backfill tool and cleanup job as production-risk code, not as optional utilities.
