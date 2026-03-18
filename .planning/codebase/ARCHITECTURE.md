# Architecture

**Analysis Date:** 2026-03-18

## Pattern Overview

**Overall:** Multi-service cloud printing platform with layered Go backend and React SPA frontend

**Key Characteristics:**
- **Layered Architecture**: Clear separation of handlers → services → repositories → database
- **Domain-Driven Design**: Models define core entities (EdgeNode, Printer, PrintJob)
- **Event-Driven Communication**: WebSocket for real-time edge node communication
- **Dual Auth Strategy**: Built-in OAuth2 or Keycloak external provider
- **Repository Pattern**: Database abstraction with PostgreSQL as backing store

## Layers

### Entry Point Layer
**Purpose:** Application bootstrap and dependency injection
- Location: `api/cmd/server/main.go`
- Contains: Wire-up of all repositories, handlers, services, and routes
- Depends on: All internal packages
- Used by: System runtime

### Handler Layer (API Layer)
**Purpose:** HTTP request handling and response formatting
- Location: `api/internal/handlers/`
- Contains: HTTP handlers for REST endpoints, request validation, response serialization
- Depends on: Repositories, WebSocket manager
- Used by: Gin router

Key handlers:
- `api/internal/handlers/printer_handler.go` - Printer CRUD and Edge registration
- `api/internal/handlers/print_job_handler.go` - Job lifecycle management
- `api/internal/handlers/edge_node_handler.go` - Edge node management
- `api/internal/handlers/oauth2_handler.go` - Authentication flows
- `api/internal/handlers/file_handler.go` - File upload/download

### Middleware Layer
**Purpose:** Cross-cutting concerns (auth, CORS, logging, security)
- Location: `api/internal/middleware/`
- Contains: OAuth2 validation, CORS, rate limiting, node status checks
- Depends on: Config, repositories
- Used by: Router setup in main.go

Key middleware:
- `api/internal/middleware/oauth2.go` - OAuth2 resource server validation
- `api/internal/middleware/common.go` - CORS, logging, security headers
- `api/internal/middleware/edge_node_check.go` - Edge node enabled/disabled validation
- `api/internal/middleware/printer_check.go` - Printer access validation

### Service Layer
**Purpose:** Business logic and orchestration
- Location: `api/internal/auth/`, `api/internal/websocket/`, `api/internal/security/`
- Contains: Authentication, WebSocket management, token generation, validation
- Depends on: Repositories, models
- Used by: Handlers

Key services:
- `api/internal/websocket/manager.go` - WebSocket connection lifecycle and broadcast
- `api/internal/websocket/connection.go` - Individual connection handling
- `api/internal/security/token_manager.go` - One-time upload/download tokens
- `api/internal/auth/builtin_auth_service.go` - Built-in OAuth2 implementation

### Repository Layer
**Purpose:** Data access abstraction
- Location: `api/internal/database/`
- Contains: CRUD operations, query builders, transaction management
- Depends on: Database connection, models
- Used by: Handlers, services

Key repositories:
- `api/internal/database/printer_repository.go` - Printer data access
- `api/internal/database/print_job_repository.go` - Job data access
- `api/internal/database/edge_node_repository.go` - Edge node data access
- `api/internal/database/user_repository.go` - User management
- `api/internal/database/file_repository.go` - File metadata

### Model Layer
**Purpose:** Domain entity definitions
- Location: `api/internal/models/`
- Contains: Struct definitions for all domain entities
- Depends on: Standard library only
- Used by: All layers

Core models:
- `api/internal/models/models.go` - EdgeNode, Printer, PrintJob, User, PrinterCapabilities
- `api/internal/models/oauth2_client.go` - OAuth2 client definitions
- `api/internal/models/file.go` - File metadata

### Infrastructure Layer
**Purpose:** External integrations and cross-cutting utilities
- Location: `api/internal/config/`, `api/internal/logger/`, `api/internal/utils/`
- Contains: Configuration loading, logging, validators
- Depends on: Environment, external libraries
- Used by: All layers

Key components:
- `api/internal/config/config.go` - Viper-based configuration with env var support
- `api/internal/logger/logger.go` - Zap structured logging
- `api/internal/utils/document_validator.go` - PDF/DOCX validation

## Data Flow

### Print Job Creation Flow:
1. **Client** → POST `/api/v1/print-jobs` (OAuth2 authenticated)
2. **Handler** (`print_job_handler.go`) validates request
3. **Repository** creates job in "pending" status
4. **WebSocket Manager** dispatches job to connected Edge node
5. **Edge Node** acknowledges, status updated to "dispatched"
6. **Edge Node** reports progress via WebSocket → status transitions to "printing" → "completed"

### Edge Node Registration Flow:
1. **Edge Agent** → POST `/api/v1/edge/register` (OAuth2 client credentials)
2. **Handler** validates and creates/updates node record
3. **Edge Agent** opens WebSocket connection at `/api/v1/edge/ws`
4. **WebSocket Handler** authenticates and registers connection
5. **Connection Manager** maintains active connection pool

### File Upload Flow:
1. **Client** → POST `/api/v1/files/preflight` (optional token)
2. **Handler** validates file and returns upload token
3. **Client** → POST `/api/v1/files` with token or OAuth2
4. **Handler** saves file to disk, stores metadata in DB
5. **Response** returns file URL with download token

## State Management

**Server State:**
- WebSocket connections stored in-memory (`websocket/manager.go`)
- Print job status persisted in PostgreSQL
- OAuth2 tokens are stateless (JWT-based)

**Background Tasks:**
- File cleanup (hourly) - deletes files older than 24h
- Token cleanup (hourly) - removes expired token records
- Stale job cleanup (30 min) - marks stuck jobs as failed
- Pending job retry (5 min) - re-dispatch pending jobs > 3min old

## Key Abstractions

**Repository Pattern:**
- Purpose: Abstract database operations
- Examples: `api/internal/database/printer_repository.go`
- Pattern: Each entity has dedicated repository with CRUD methods

**WebSocket Connection Management:**
- Purpose: Real-time bidirectional communication with edge nodes
- Examples: `api/internal/websocket/manager.go`, `connection.go`
- Pattern: Manager maintains connection map with goroutine-safe access

**OAuth2 Resource Server:**
- Purpose: Protect API endpoints with scope-based access control
- Examples: `api/internal/middleware/oauth2.go`
- Pattern: Middleware validates JWT and extracts claims

**One-Time Token System:**
- Purpose: Secure file access without session management
- Examples: `api/internal/security/token_manager.go`
- Pattern: HMAC-signed tokens with expiration and single-use enforcement

## Entry Points

**API Server:**
- Location: `api/cmd/server/main.go`
- Triggers: Direct execution or Docker container start
- Responsibilities: Config loading, DB initialization, route setup, background task scheduling

**Admin Console (Frontend):**
- Location: `admin/src/index.tsx`
- Triggers: Browser navigation to `/`
- Responsibilities: React app mount, routing, authentication check

**WebSocket Endpoint:**
- Location: `api/internal/websocket/handler.go`
- Triggers: WebSocket upgrade request to `/api/v1/edge/ws`
- Responsibilities: Connection upgrade, authentication, message handling

## Error Handling

**Strategy:** Centralized error types with HTTP status mapping

**Patterns:**
- Handler errors return standardized JSON: `{"code": N, "message": "...", "data": null}`
- Repository errors wrap with context: `fmt.Errorf("failed to X: %w", err)`
- WebSocket errors logged but don't crash connection manager
- Validation errors return 400 with field-level details

**Key Files:**
- `api/internal/handlers/errors.go` - HTTP error responses
- `api/internal/websocket/errors.go` - WebSocket error types

## Cross-Cutting Concerns

**Logging:**
- Framework: Uber Zap
- Pattern: Structured JSON logging with contextual fields
- Location: `api/internal/logger/logger.go`

**Validation:**
- Framework: go-playground/validator
- Pattern: Struct tags + custom validators
- Location: Handlers define request structs with validation tags

**Authentication:**
- Strategy: JWT (built-in) or Keycloak (external)
- Pattern: Middleware extracts and validates tokens, injects claims into context
- Location: `api/internal/middleware/oauth2.go`

---

*Architecture analysis: 2026-03-18*
