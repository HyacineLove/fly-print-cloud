# Cloud MinIO Storage Decoupling Design

**Date:** 2026-05-21

## Goal

Decouple cloud file storage from the local filesystem and move uploaded print files to a pluggable storage layer with MinIO as the primary object-storage backend, while preserving the current upload, preview, and Edge print flow.

This phase must keep the public flow stable:

- upload still enters through `POST /api/v1/files`
- preview and print dispatch still reference the same uploaded `file_id`
- Edge can continue to fetch files without requiring a broad protocol rewrite

## Non-Goals

- Do not redesign the upload token or download token protocol in this phase.
- Do not require a mandatory Edge-side refactor for the first rollout.
- Do not switch the preview/print flow to direct presigned URLs in the first rollout.
- Do not remove local storage support entirely; keep it as a compatibility backend.

## Current State

### Cloud storage coupling

The current cloud implementation is tightly bound to local disk storage:

- uploads create the local directory and save files directly under `storage.upload_dir`
- file metadata stores a local path semantic in `files.file_path`
- downloads serve the file by calling `c.FileAttachment(file.FilePath, ...)`
- file cleanup deletes local files with `os.Remove`
- `docker-compose` mounts a persistent `file_uploads` volume for the API container

### WebSocket and Edge expectations

The cloud currently dispatches file references to Edge using `/api/v1/files/{id}` and a one-time download token.

This is important because the Edge code already supports:

- relative cloud file URLs that are expanded with the configured `base_url`
- tokenized API downloads via query parameter
- direct S3-compatible presigned URLs in the print-download path

This means the first MinIO rollout can keep the download API stable and move storage behind the API boundary.

## Desired Architecture

### Storage abstraction

Introduce a storage service layer inside the cloud API:

- `StorageService`
- `LocalStorageBackend`
- `MinIOStorageBackend`

The API handlers and cleanup jobs must depend on the storage abstraction rather than on filesystem calls.

For the local backend, migration-period compatibility must include both:

- new relative object keys such as `uuid.pdf`
- legacy `files.file_path` values that already include `storage.upload_dir` or an absolute path under that root

Without that dual-read behavior, cleanup and backfill can fail on pre-migration uploads even if the new abstraction is otherwise correct.

### Phase 1 serving model

The first rollout should use **proxy download mode**:

- uploaded files are stored in MinIO
- file metadata points to MinIO object identity
- `GET /api/v1/files/:id` streams the object back through the cloud API after auth/token validation
- WebSocket `file_url` remains `/api/v1/files/{id}`

This keeps Edge stable and limits rollout risk.

### Phase 2 serving model

After the MinIO-backed proxy path is stable, optionally add **presigned download mode**:

- cloud generates short-lived presigned GET URLs
- WebSocket `file_url` can be the presigned MinIO URL directly
- Edge preview and print download can bypass the cloud API for object payload transfer

This is an optimization phase, not part of the minimum safe migration.

## Configuration Design

Extend cloud storage config with backend selection and MinIO connection settings.

Example shape:

```yaml
storage:
  provider: "local"          # local | minio
  upload_dir: "./uploads"    # local backend only
  max_size: 10485760
  max_document_pages: 5
  download_mode: "proxy"     # proxy | presigned
  minio:
    endpoint: "minio:9000"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    bucket: "fly-print-files"
    use_ssl: false
    object_prefix: "uploads/"
```

Rules:

- `provider=local` remains the default for compatibility.
- `download_mode=proxy` remains the default for the first MinIO rollout.
- MinIO config is only required when `provider=minio`.

## Data Model Design

### Files table

Do not keep overloading `files.file_path` with mixed semantics if it can be avoided.

Recommended migration:

- keep existing fields temporarily
- add:
  - `storage_provider`
  - `storage_bucket`
  - `object_key`

Semantics:

- `storage_provider` indicates `local` or `minio`
- `storage_bucket` is the target bucket for object storage backends
- `object_key` is the canonical storage locator
- `file_path` is retained temporarily for backward compatibility and migration fallback

### Print jobs table

`print_jobs.file_url` already carries the main transport identity used by Edge and can stay.

`print_jobs.file_path` should no longer be treated as a required local disk path for cloud-originated jobs. It can be retained for compatibility, but the source of truth should remain the `files` table plus `file_url`.

## Storage Interface

The storage abstraction should cover the current cloud needs:

- `Put(ctx, objectKey, reader, size, contentType) error`
- `Get(ctx, objectKey) (ReadCloser, metadata, error)`
- `Delete(ctx, objectKey) error`
- `Stat(ctx, objectKey) (metadata, error)`
- `GeneratePresignedGet(ctx, objectKey, ttl) (string, error)` for phase 2

The interface should also support content type and object size metadata so download responses remain correct.

## Upload Flow

Target phase-1 upload sequence:

1. Receive multipart upload through `POST /api/v1/files`
2. Validate token/auth exactly as today
3. Validate size, file type, and document page count exactly as today
4. Generate a stable object key
5. Stream the file into the configured backend
6. Persist metadata in `files`
7. Dispatch preview to Edge using the same `file_id`

The upload validation order must remain unchanged so the current bug fixes and upload-policy behavior are preserved.

## Download Flow

Target phase-1 download sequence:

1. Validate download token or OAuth2 auth exactly as today
2. Load file metadata from `files`
3. Resolve the backend from `storage_provider`
4. Stream object content from storage backend to the client
5. Return original filename in `Content-Disposition`

This keeps the contract stable for both admin users and Edge.

## Preview and Print Dispatch

Phase 1:

- `DispatchPreviewFile` continues to send `/api/v1/files/{id}`
- `DispatchPrintJob` continues to send `/api/v1/files/{id}`
- cloud download token generation remains unchanged

Phase 2 optional optimization:

- when `download_mode=presigned`, preview/print dispatch may carry a MinIO presigned URL
- download token handling must be re-evaluated so the presigned URL lifetime and current token TTLs do not conflict

## Cleanup Strategy

Current cleanup is local-file based and must be refactored.

New behavior:

- list stale file records from DB
- delete the corresponding object through the storage backend
- then delete the DB record

During migration, cleanup must tolerate mixed storage backends.

## Migration Strategy

### Backward-compatible rollout

Rollout should happen in this order:

1. Add schema fields and storage abstraction
2. Deploy code that supports both `local` and `minio`
3. Keep production on `provider=local`
4. Backfill old local files into MinIO
5. Switch config to `provider=minio`
6. Observe upload, preview, print, retry-dispatch, and cleanup paths
7. Remove old local volume only after the migration is proven complete

### Backfill behavior

Backfill tool responsibilities:

- query files that are still `local`
- read the old local file
- upload to MinIO with a deterministic object key
- update file metadata to `minio`
- record failures without aborting the whole batch

The tool must be idempotent.

## Docker Compose and Standalone Deployment

### Docker Compose

Add:

- `minio` service
- optional one-shot `mc` init service to create bucket and set policy

The API service receives MinIO config through environment variables.

### Standalone deployment

Keep standalone API deployment supported through `config.yaml` and `FLY_PRINT_*` env vars.

This means the code must not assume Docker networking or compose-only hostnames.

## Validation and Testing

### Backend tests

Must cover:

- local backend still works
- MinIO backend upload works
- MinIO-backed download through `/api/v1/files/:id` works
- cleanup deletes MinIO objects correctly
- old local records can still be read during migration
- config validation rejects incomplete MinIO config when `provider=minio`

### Integration verification

Must manually verify:

1. QR upload still succeeds
2. Edge preview still loads
3. `submit_print_params` still creates a job and Edge downloads the file
4. pending-job redispatch still works after Edge reconnect
5. cleanup removes expired files from the active backend
6. compose startup works with MinIO enabled

## Risks

- Overloading `file_path` with object-key semantics will create long-term confusion if not cleaned up.
- Directly switching WebSocket payloads to presigned URLs in phase 1 would expand risk unnecessarily.
- Cleanup and migration code can orphan objects or records if delete/update ordering is wrong.
- Edge preview and print paths are similar but not identical, so both must be verified explicitly.

## Acceptance Criteria

This design is complete when all of the following are true:

- cloud file storage no longer requires local disk when `provider=minio`
- upload, preview, and print still work through the current QR + Edge flow
- the API can run with either `local` or `minio` storage backend
- the compose stack can boot with MinIO included
- old records can be migrated without breaking existing downloads
- cleanup works for the configured backend
