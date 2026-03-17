# Codebase Concerns

**Analysis Date:** 2025-03-17

## Tech Debt

### Hardcoded Values
- **File upload constraints**: Hardcoded constants in `api/internal/handlers/file_handler.go` lines 25-28:
  - `uploadRuleMaxSizeBytes = 10 * 1024 * 1024` (10MB limit)
  - `uploadRuleMaxPages = 5` (5 page limit)
  - These should be configurable via `config.StorageConfig` but are hardcoded

- **WebSocket timeout values**: Hardcoded in `api/internal/websocket/manager.go`:
  - Line 70: `time.Sleep(500 * time.Millisecond)` - connection stabilization delay
  - Line 90: `time.Sleep(100 * time.Millisecond)` - job dispatch throttling
  - Line 329: `10*time.Second` - ACK timeout for print job dispatch
  - Line 199: `time.Sleep(100 * time.Millisecond)` - disconnect notification delay

- **Database connection pool**: Hardcoded in `api/internal/database/database.go` lines 37-39:
  - `SetMaxOpenConns(25)`
  - `SetMaxIdleConns(5)`
  - `SetConnMaxLifetime(5 * time.Minute)`

- **Edge node offline timeout**: Hardcoded 3 minutes in multiple locations:
  - `api/internal/handlers/edge_node_handler.go` line 198
  - Used in `CheckAndUpdateOfflineNodes(3)` calls

- **Token TTL defaults**: Hardcoded in `api/internal/security/token_manager.go` line 25:
  - `DefaultTokenTTL = 180` (3 minutes)

### Error String Matching
- **Fragile error checking**: In `api/internal/security/token_manager.go` lines 270-275 and 361-367:
  - Error messages compared by string: `err.Error() == "token has already been used"`
  - Error messages compared by string: `err.Error() == "token has been revoked"`
  - Should use typed errors or error codes instead

### SQL String Building
- **Dynamic SQL construction**: In multiple repository files:
  - `api/internal/database/edge_node_repository.go` lines 285-317: Dynamic ORDER BY and LIMIT clauses built with string concatenation
  - `api/internal/database/print_job_repository.go` lines 104-155: Dynamic WHERE clause building with fmt.Sprintf
  - While parameterized queries are used for values, the structure is still dynamically built

### TODO Comment
- **Missing error logging**: In `admin/src/components/ErrorBoundary.tsx` line 44:
  - `// TODO: 可以在这里发送错误日志到服务器`
  - Error boundary logs to console but never sends to server

## Known Issues

### Console/Debug Logging in Production
- **Excessive console logging**: Multiple `console.log` and `console.error` calls in admin React code:
  - `admin/src/components/pages/EdgeNodes.tsx`: Lines 76, 81, 84, 87 (debug logging with emojis)
  - `admin/src/components/pages/*.tsx`: Multiple components log errors to console
  - These should be removed or use a proper logging service in production

- **Backend fmt.Printf usage**: In `api/internal/handlers/file_handler.go` line 177:
  - `fmt.Printf("Failed to dispatch preview to node %s: %v\n", nodeID, err)`
  - Should use structured logger (zap) like other parts of codebase

- **OAuth2 handler fmt.Printf**: In `api/internal/handlers/oauth2_handler.go` lines 244, 259:
  - User sync errors printed with fmt.Printf instead of logger

### JWT Token Parsing Without Verification
- **Security concern**: In `api/internal/middleware/oauth2.go` line 120-121:
  - `parser := jwt.NewParser(jwt.WithoutClaimsValidation())`
  - Token signature not verified when parsing JWT
  - Relies on Keycloak UserInfo endpoint for actual validation
  - Comment acknowledges this: "解析 JWT token（跳过签名验证）"

### Missing Context Cancellation
- **HTTP client without timeout**: In `api/internal/middleware/oauth2.go` line 200:
  - `http.DefaultClient.Do(req)` uses default client without timeout
  - Could hang indefinitely if UserInfo endpoint is unresponsive

### Potential Resource Leak
- **File handle in file handler**: In `api/internal/handlers/file_handler.go`:
  - Line 100: `defer srcFile.Close()` inside Upload handler
  - If `c.FormFile("file")` succeeds but `fileHeader.Open()` fails, the original form file may not be closed
  - Actually uses `srcFile` which is from `fileHeader.Open()`, but error handling at line 96-99 doesn't close

### Incomplete Error Handling
- **Silent failures**: In `api/internal/database/database.go` lines 316-320:
  - Migration errors logged as warnings but don't stop execution
  - Could lead to inconsistent database state

## Security Concerns

### Default Secrets in Code
- **Development-only secrets**: In `api/internal/config/config.go`:
  - Line 261: `viper.SetDefault("oauth2.jwt_signing_secret", "fly-print-jwt-secret-dev-only")`
  - Line 284: `viper.SetDefault("security.file_access_secret", "fly-print-file-access-secret-dev-only")`
  - While validation exists in `Validate()` method, defaults are weak

### Token Manager Default Secret
- **Fallback to insecure default**: In `api/internal/security/token_manager.go` lines 90-92:
  - `if secret == "" { secret = DefaultSecret }`
  - `DefaultSecret = "fly-print-file-access-secret-dev-only"`
  - Should fail closed rather than fallback to weak default

### SQL Injection Risk (Low)
- **Dynamic ORDER BY**: In `api/internal/database/edge_node_repository.go` lines 280-297:
  - `orderClause` built from user input (`sortBy` parameter)
  - While validated against whitelist, still risky pattern
  - Similar pattern in `ListEdgeNodes` query building

### Missing Input Sanitization
- **MAC address not validated**: In `api/internal/handlers/edge_node_handler.go`:
  - MAC address stored from request without format validation
  - Should validate against pattern like `^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`

- **Filename sanitization limited**: In `api/internal/security/validation.go` lines 164-175:
  - `SanitizeFilename` only removes dangerous chars but doesn't prevent path traversal completely
  - Should also validate against known safe patterns

### CORS Configuration
- **Overly permissive defaults**: In `api/internal/config/config.go` lines 251-257:
  - Multiple localhost origins allowed by default
  - Includes `http://localhost:8012` for development convenience
  - Could be exploited in development environments

## Performance Issues

### Database Query Inefficiency
- **N+1 query pattern**: In `api/internal/handlers/edge_node_handler.go` lines 210-214:
  - `printerRepo.CountPrintersByEdgeNode(node.ID)` called in loop for each node
  - Should use JOIN or batch count query
  - Similar pattern in lines 259-262

- **Pagination without cursor**: In `api/internal/database/print_job_repository.go`:
  - Standard OFFSET/LIMIT pagination used
  - Performance degrades with large offsets
  - Should consider keyset pagination for large tables

### Memory Usage
- **File upload to memory**: In `api/internal/handlers/file_handler.go`:
  - `c.SaveUploadedFile()` likely buffers entire file
  - No streaming upload support
  - Could exhaust memory with concurrent large uploads

### WebSocket Message Broadcasting
- **No message queue**: In `api/internal/websocket/manager.go` line 122-135:
  - `broadcastMessage` iterates all connections synchronously
  - Slow consumer could block broadcast to other nodes
  - No backpressure handling

## Fragile Areas

### Race Conditions
- **Token revocation race**: In `api/internal/security/token_manager.go`:
  - Lines 112-126: Old tokens revoked before new token generated
  - Lines 162-176: Similar race for download tokens
  - If revoke succeeds but generation fails, node has no valid tokens

- **WebSocket connection replacement**: In `api/internal/websocket/manager.go` lines 59-64:
  - Old connection closed after new connection registered
  - Brief window where both could exist

### Inconsistent Error Handling
- **Mixed error response formats**: Throughout API:
  - Some handlers use `ErrorResponseWithCode` with numeric codes
  - Others return raw gin.H{"error": "message"}
  - Some return localized Chinese messages, others English

- **Silent failure pattern**: In `api/internal/handlers/print_job_handler.go` lines 203-213:
  - Print job dispatch failure logged but not returned to client
  - Client receives 201 Created but job may not actually dispatch

### Magic Numbers
- **Status code confusion**: In `api/internal/handlers/file_handler.go`:
  - Line 14: Hardcoded `len("/api/v1/files/") = 14` for file ID extraction
  - Same in `api/internal/websocket/manager.go` lines 238-240, 294-296
  - Should use constant or strings.LastIndex

## Missing Critical Features

### No Rate Limiting
- **Missing protection**: No rate limiting found on:
  - Token generation endpoints
  - File upload endpoints
  - Edge node registration
  - OAuth2 endpoints

### No Request Timeout
- **Missing timeouts**: HTTP handlers don't set request timeouts
- Could allow long-running requests to consume resources

### No Input Size Validation
- **Missing limits**: Most request structs don't have size limits on string fields
- Could allow DoS via very large payloads

### No Audit Logging
- **Missing audit trail**: No centralized audit logging for:
  - Administrative actions (create/delete edge nodes)
  - Print job submissions
  - File uploads/downloads
  - Authentication events

## Test Coverage Gaps

### No Test Files Found
- **Missing tests**: No `*_test.go` files found in API codebase
- **No React tests**: No `*.test.tsx` or `*.spec.tsx` files found

### Critical Paths Untested
- **Token validation logic**: Complex one-time token flow has no automated tests
- **WebSocket message handling**: Connection lifecycle not tested
- **OAuth2 integration**: Both builtin and Keycloak modes need tests
- **File upload validation**: Magic byte detection and page counting

## Recommendations

### Immediate (High Priority)
1. Remove or secure all development-only secrets
2. Add rate limiting to authentication and file upload endpoints
3. Fix JWT signature verification in middleware
4. Add proper error handling for WebSocket dispatch failures

### Short Term
1. Make all hardcoded timeouts configurable
2. Add comprehensive audit logging
3. Implement request timeouts on all handlers
4. Add database query optimization for N+1 patterns

### Long Term
1. Add comprehensive test suite (unit, integration, e2e)
2. Implement message queue for WebSocket broadcasts
3. Add streaming file upload support
4. Consider implementing cursor-based pagination

---

*Concerns audit: 2025-03-17*
