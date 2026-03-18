# Codebase Concerns

**Analysis Date:** 2025-03-18

## Tech Debt

### JWT Token Signature Verification Bypass
- **Issue:** JWT tokens are parsed without signature verification in `parseJWTToken()` function
- **File:** `api/internal/middleware/oauth2.go` (lines 118-181)
- **Code:** Uses `jwt.WithoutClaimsValidation()` and `parser.ParseUnverified()`
- **Impact:** Client Credentials Flow tokens are accepted without cryptographic verification, enabling token forgery
- **Fix approach:** Add proper JWT signature verification using JWKS endpoint or shared secret

### Type Assertion Error Handling Gaps
- **Issue:** Multiple type assertions without proper ok-check patterns
- **Files:**
  - `api/internal/websocket/connection.go` (lines 271-293, msg.Data type assertions)
  - `api/internal/middleware/oauth2.go` (lines 134-145, claims extraction)
- **Impact:** Panic on malformed input instead of graceful error handling
- **Fix approach:** Use `value, ok := interface.(Type)` pattern consistently with proper error returns

### Database Connection String in DSN
- **Issue:** Password visible in connection string returned by `GetDSN()`
- **File:** `api/internal/config/config.go` (lines 290-293)
- **Impact:** Credentials may be logged if DSN is printed for debugging
- **Fix approach:** Use connection string with environment variables or secrets manager

### Hardcoded Default Secrets
- **Issue:** Default secrets present in code for development convenience
- **Files:**
  - `api/internal/config/config.go` (line 261, 284: default JWT and file access secrets)
  - `api/internal/security/token_manager.go` (line 28: DefaultSecret constant)
- **Impact:** Production deployments may accidentally use weak default secrets
- **Fix approach:** Enforce mandatory secrets in production (validation already present, but defaults still in code)

## Known Issues

### WebSocket Connection ACK Timeout Handling
- **Issue:** ACK timeout for print job dispatch rolls back status but race condition possible
- **File:** `api/internal/websocket/manager.go` (lines 327-344)
- **Symptoms:** Job status may be inconsistent if ACK arrives after timeout but before rollback
- **Trigger:** Network latency > 10 seconds during print job dispatch
- **Workaround:** Automatic retry mechanism exists but may cause duplicate job dispatch

### File Upload Token Race Condition
- **Issue:** Two-phase token validation in file upload allows TOCTOU attack window
- **File:** `api/internal/handlers/file_handler.go` (lines 46-150)
- **Symptoms:** Token validated twice - lightweight check before file save, full validation after
- **Impact:** Malicious user could potentially replay token between phases
- **Fix approach:** Single atomic token validation or use single-use token bucket

### Database Query N+1 Pattern
- **Issue:** Printer listing queries edge node status individually for each printer
- **File:** `api/internal/handlers/printer_handler.go` (lines 217-229)
- **Impact:** O(n) database queries for n printers, performance degradation at scale
- **Fix approach:** Use JOIN query or batch fetch edge node statuses

### Memory Store Rate Limiter
- **Issue:** Rate limiter uses in-memory store without persistence
- **File:** `api/cmd/server/main.go` (lines 166-168)
- **Impact:** Rate limits reset on server restart, not shared across replicas
- **Fix approach:** Use Redis-backed store for distributed rate limiting

## Security Considerations

### CORS Wildcard Origins in Development
- **Risk:** Development configuration allows broad private network CORS origins
- **File:** `.env.example` (line 93)
- **Current mitigation:** Only affects development mode, production requires explicit configuration
- **Recommendations:** Add CORS origin validation warnings in deployment checklist

### File Upload Path Traversal
- **Risk:** File upload uses original filename without full sanitization
- **File:** `api/internal/handlers/file_handler.go` (line 120)
- **Current mitigation:** `security.SanitizeFilename()` function exists but implementation not verified
- **Recommendations:** Audit `SanitizeFilename` implementation for path traversal protection

### SQL Injection via Dynamic Queries
- **Risk:** Raw SQL construction in database migrations
- **File:** `api/internal/database/database.go` (lines 307-321, migrations)
- **Current mitigation:** Migrations are hardcoded, not user-input
- **Recommendations:** Use parameterized queries consistently

### Missing Request Body Size Limits
- **Risk:** No explicit request body size limit on non-file endpoints
- **Impact:** Potential DoS via large JSON payloads
- **Recommendations:** Add middleware to limit request body size for all routes

## Performance Bottlenecks

### Synchronous Database Operations
- **Problem:** All database operations are synchronous, blocking goroutines
- **Files:** All repository files in `api/internal/database/`
- **Cause:** Uses standard `database/sql` without context cancellation support
- **Improvement path:** Add context support to all DB operations for timeout/cancellation

### WebSocket Broadcast Lock Contention
- **Problem:** Broadcast operations hold read lock while iterating all connections
- **File:** `api/internal/websocket/manager.go` (lines 122-135)
- **Cause:** `broadcastMessage()` locks mutex during full iteration
- **Improvement path:** Use lock-free channels or sharded connection maps

### File Upload Validation Reads Entire File
- **Problem:** Page count validation reads entire PDF into memory
- **File:** `api/internal/utils/document_validator.go` (lines 54-67)
- **Cause:** `pdfcpu` library may buffer large files
- **Improvement path:** Stream validation or size limits before validation

## Fragile Areas

### WebSocket Connection Lifecycle Management
- **Files:**
  - `api/internal/websocket/connection.go` (lines 70-117, ReadPump/WritePump)
  - `api/internal/websocket/manager.go` (lines 54-118, register/unregister)
- **Why fragile:** Complex goroutine coordination with multiple channel operations
- **Safe modification:** Always use defer for cleanup, respect channel closing patterns
- **Test coverage:** No unit tests found for WebSocket connection handling

### OAuth2 Token Validation Dual-Path
- **File:** `api/internal/middleware/oauth2.go` (lines 107-115)
- **Why fragile:** Two different validation paths (JWT parse vs UserInfo) with different behavior
- **Safe modification:** Ensure both paths extract identical claims and scopes
- **Test coverage:** Limited test coverage for token edge cases

### Database Transaction Error Handling
- **File:** `api/internal/database/database.go` (lines 64-87)
- **Why fragile:** Generic error wrapping may hide underlying PostgreSQL error codes
- **Safe modification:** Check for specific error types and handle accordingly
- **Test coverage:** No transaction rollback scenario tests

### Job Retry Mechanism State Machine
- **File:** `api/cmd/server/main.go` (lines 431-531)
- **Why fragile:** Complex state transitions between pending/dispatched/printing/completed/failed
- **Safe modification:** Use explicit state machine with validation
- **Test coverage:** No retry scenario tests

## Scaling Limits

### Single WebSocket Manager Instance
- **Current capacity:** Single goroutine manages all connections
- **Limit:** Memory and goroutine limits of single process
- **Scaling path:** Shard connections by node_id across multiple manager instances

### In-Memory Rate Limiter
- **Current capacity:** ~10 req/s per instance (configurable)
- **Limit:** Memory usage scales with unique client count
- **Scaling path:** Use Redis or other distributed store

### File Upload Directory
- **Current capacity:** Local filesystem
- **Limit:** Single server disk space
- **Scaling path:** Use object storage (S3, MinIO) for file storage

### PostgreSQL Connection Pool
- **Current configuration:** 25 max open connections (hardcoded in `database.go` line 37)
- **Limit:** Database server connection limit
- **Scaling path:** Use connection pooler (PgBouncer) or increase pool size

## Dependencies at Risk

### pdfcpu (PDF processing)
- **Risk:** Heavy dependency for simple page count validation
- **Impact:** Large binary size, potential memory issues with malformed PDFs
- **Migration plan:** Consider lighter PDF libraries or service-based validation

### gorilla/websocket (WebSocket library)
- **Risk:** Project maintenance status - gorilla toolkit is community maintained
- **Impact:** Future security patches may be delayed
- **Migration plan:** Monitor for alternatives (nhooyr/websocket) but stable for now

### ulule/limiter (Rate limiting)
- **Risk:** Smaller project, limited maintenance
- **Impact:** Feature gaps for distributed rate limiting
- **Migration plan:** Evaluate tollbooth or Redis-based solutions

## Missing Critical Features

### Health Check Deep Validation
- **Problem:** `/health` endpoint returns basic status without dependency checks
- **File:** `api/internal/handlers/health_handler.go`
- **What's missing:** Database connectivity, WebSocket status, external service health

### Request Logging Middleware
- **Problem:** No structured request logging with timing
- **What's missing:** HTTP request/response logging, latency tracking, request ID propagation

### Database Migration Tooling
- **Problem:** Schema migrations embedded in application startup
- **File:** `api/internal/database/database.go` (lines 307-321)
- **What's missing:** Versioned migrations, rollback capability, migration locking

### Circuit Breaker Pattern
- **Problem:** No protection against cascading failures
- **What's missing:** Circuit breaker for external calls, especially OAuth2 UserInfo endpoint

## Test Coverage Gaps

### No Test Files Found
- **Status:** Zero `*_test.go` files in entire codebase
- **High-risk untested areas:**
  - WebSocket connection handling (`api/internal/websocket/`)
  - OAuth2 token validation (`api/internal/middleware/oauth2.go`)
  - Database repository operations (`api/internal/database/*_repository.go`)
  - File upload/download handlers (`api/internal/handlers/file_handler.go`)
  - Token generation and validation (`api/internal/security/token_manager.go`)

### Missing Integration Tests
- **Untested scenarios:**
  - End-to-end print job flow
  - Edge node registration and heartbeat
  - File upload with token validation
  - OAuth2 authentication flows (both builtin and Keycloak modes)

### Manual Testing Only Areas
- **Document page validation** (`api/internal/utils/document_validator.go`)
- **Database transaction handling**
- **WebSocket reconnection and message retry**

## Documentation Gaps

### API Documentation
- **File:** `api/docs/docs.go` (Swagger generated)
- **Gap:** No manual API documentation for edge cases and error responses

### Deployment Documentation
- **Gap:** No production deployment checklist or security hardening guide

### Architecture Decision Records
- **Gap:** No ADRs documenting why specific patterns were chosen

---

*Concerns audit: 2025-03-18*
