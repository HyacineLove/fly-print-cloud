# Architecture

**Analysis Date:** 2025-03-17

## Pattern Overview

**Overall:** Layered Clean Architecture with Repository Pattern

**Key Characteristics:**
- **Backend (Go):** MVC-inspired layered architecture with clear separation of concerns
- **Frontend (React):** Component-based SPA with container/presentational pattern
- **Real-time Communication:** WebSocket-based bidirectional messaging for edge node communication
- **Authentication:** Dual-mode OAuth2 (builtin JWT or external Keycloak)

## Layers

### API Layer (HTTP/REST Handlers)
- **Purpose:** Handle HTTP requests/responses, input validation, response formatting
- **Location:** `api/internal/handlers/`
- **Contains:** Gin HTTP handlers for all API endpoints
- **Depends on:** Repository Layer, Security Layer
- **Used by:** Nginx reverse proxy, Admin UI, Edge Nodes

**Key Handlers:**
- `api/internal/handlers/user_handler.go` - User CRUD operations
- `api/internal/handlers/edge_node_handler.go` - Edge node management
- `api/internal/handlers/printer_handler.go` - Printer management
- `api/internal/handlers/print_job_handler.go` - Print job lifecycle
- `api/internal/handlers/oauth2_handler.go` - Authentication flows
- `api/internal/handlers/file_handler.go` - File upload/download
- `api/internal/handlers/health_handler.go` - Health checks
- `api/internal/handlers/dashboard_handler.go` - Analytics/metrics

### Repository Layer (Database Access)
- **Purpose:** Abstract database operations, implement data persistence logic
- **Location:** `api/internal/database/`
- **Contains:** Repository structs with SQL queries
- **Depends on:** Database connection (PostgreSQL)
- **Used by:** Handler Layer, Background tasks

**Key Repositories:**
- `api/internal/database/user_repository.go` - User data access
- `api/internal/database/edge_node_repository.go` - Edge node data
- `api/internal/database/printer_repository.go` - Printer data
- `api/internal/database/print_job_repository.go` - Print job data
- `api/internal/database/file_repository.go` - File metadata
- `api/internal/database/token_usage_repository.go` - One-time token tracking

### Domain/Model Layer
- **Purpose:** Define core business entities and data structures
- **Location:** `api/internal/models/`
- **Contains:** Struct definitions with JSON tags
- **Key Files:**
  - `api/internal/models/models.go` - Core entities (User, Printer, PrintJob, EdgeNode)
  - `api/internal/models/file.go` - File-related models
  - `api/internal/models/oauth2_client.go` - OAuth2 client models

### Security Layer
- **Purpose:** Authentication, authorization, token management
- **Location:** `api/internal/security/`, `api/internal/auth/`
- **Contains:** 
  - JWT token generation/validation
  - OAuth2 client credential verification
  - Password hashing (bcrypt)
  - One-time token management for file access

**Key Components:**
- `api/internal/security/token_manager.go` - One-time upload/download tokens
- `api/internal/security/validation.go` - Input validation
- `api/internal/auth/builtin_auth_service.go` - Built-in OAuth2 service
- `api/internal/middleware/oauth2.go` - OAuth2 resource server middleware

### WebSocket Layer (Real-time Communication)
- **Purpose:** Bidirectional communication with edge nodes
- **Location:** `api/internal/websocket/`
- **Contains:** Connection management, message routing, command dispatching

**Key Components:**
- `api/internal/websocket/manager.go` - Connection registry and broadcast
- `api/internal/websocket/handler.go` - WebSocket upgrade handling
- `api/internal/websocket/connection.go` - Individual connection management
- `api/internal/websocket/message.go` - Message protocol definitions

### Middleware Layer
- **Purpose:** Cross-cutting concerns (auth, CORS, logging, rate limiting)
- **Location:** `api/internal/middleware/`
- **Key Files:**
  - `api/internal/middleware/oauth2.go` - OAuth2 validation
  - `api/internal/middleware/edge_node_check.go` - Edge node state checks
  - `api/internal/middleware/printer_check.go` - Printer state checks
  - `api/internal/middleware/common.go` - Logger, CORS, security headers

### Configuration Layer
- **Purpose:** Application configuration management
- **Location:** `api/internal/config/`
- **Key File:** `api/internal/config/config.go` - Viper-based config with env var support

### Admin UI Layer (React Frontend)
- **Purpose:** Management console for administrators
- **Location:** `admin/src/`
- **Pattern:** Functional components with hooks, Ant Design UI library

**Key Structure:**
- `admin/src/App.tsx` - Root component with routing
- `admin/src/components/pages/` - Page components (Dashboard, Users, Printers, etc.)
- `admin/src/services/api.ts` - API client service
- `admin/src/utils/errorHandler.ts` - Error handling utilities

## Data Flow

### Print Job Lifecycle:

1. **Submission:** Third-party API → `POST /api/v1/print-jobs` → `PrintJobHandler.CreatePrintJob()`
2. **Persistence:** Handler → `PrintJobRepository.Create()` → PostgreSQL
3. **Dispatch:** Handler → `WebSocketManager.DispatchPrintJob()` → Edge Node (via WebSocket)
4. **Execution:** Edge Node downloads file (using one-time token) → Prints → Reports status via WebSocket
5. **Status Update:** WebSocket → `Connection.handlePrintJobStatus()` → `PrintJobRepository.Update()`

### File Upload Flow:

1. **Preflight:** Client → `POST /api/v1/files/preflight` → Validation → Upload token generated
2. **Upload:** Client → `POST /api/v1/files?token={token}` → File saved → Token marked used
3. **Download:** Edge Node → `GET /api/v1/files/{id}?token={token}` → File streamed → Token consumed

### Edge Node Registration:

1. **Register:** Edge Node → `POST /api/v1/edge/register` (OAuth2: edge:register scope)
2. **WebSocket Connect:** Edge Node → `GET /api/v1/edge/ws` (JWT in query param)
3. **Heartbeat:** Continuous WebSocket ping/pong (no HTTP heartbeat)
4. **Printer Sync:** Edge Node registers printers via WebSocket or REST API

### Authentication Flow (Builtin Mode):

1. **Login:** User → `GET /auth/login` → Redirect to built-in login
2. **Token Exchange:** Credentials → `POST /auth/token` → JWT access token
3. **API Access:** Client includes `Authorization: Bearer {token}` header
4. **Validation:** `OAuth2ResourceServer` middleware validates JWT signature and scope

## Key Abstractions

### Repository Pattern
- **Purpose:** Abstract data access, enable testing with mocks
- **Implementation:** Each entity has a repository struct with methods like `GetByID`, `List`, `Create`, `Update`, `Delete`
- **Example:** `api/internal/database/user_repository.go`

### WebSocket Connection Manager
- **Purpose:** Manage edge node connections, enable real-time job dispatch
- **Pattern:** Hub-and-spoke with channels for register/unregister/broadcast
- **Location:** `api/internal/websocket/manager.go`

### Token Manager (One-time Tokens)
- **Purpose:** Secure file access without session management
- **Flow:** Generate signed token → Store hash in DB → Validate on use → Mark consumed
- **Location:** `api/internal/security/token_manager.go`

### Response Helpers
- **Purpose:** Consistent API response format
- **Pattern:** Standardized JSON structure with code/message/data
- **Location:** `api/internal/handlers/response.go`

## Entry Points

### Backend Entry Point
- **Location:** `api/cmd/server/main.go`
- **Responsibilities:**
  - Configuration loading (`config.Load()`)
  - Database initialization (`database.New()`)
  - Repository instantiation
  - Handler initialization with dependency injection
  - Route setup (`setupRoutes()`)
  - Background task scheduling (token cleanup, file cleanup, job retry)
  - Graceful shutdown handling

### Frontend Entry Point
- **Location:** `admin/src/index.tsx`
- **Responsibilities:**
  - React DOM rendering
  - App component mounting

### WebSocket Entry Point
- **Location:** `api/internal/websocket/handler.go` (HTTP upgrade)
- **Handler:** `WebSocketHandler.HandleConnection()`
- **Process:** Validate token → Upgrade to WebSocket → Register with manager → Start read/write loops

## Service Boundaries

### Cloud API Service (Go/Gin)
**Responsibilities:**
- HTTP API endpoints for all operations
- OAuth2 resource server (authentication/authorization)
- WebSocket server for edge node communication
- File storage and token management
- Database persistence

**Deployment:** Docker container (`api` service)

### Admin Console (React/TypeScript)
**Responsibilities:**
- User management interface
- Edge node monitoring
- Printer configuration
- Print job tracking
- OAuth2 client management (builtin mode)
- Dashboard with metrics

**Deployment:** Built as static files, served by Nginx

### Edge Node Agent (External)
**Communication:**
- WebSocket persistent connection to Cloud API
- OAuth2 client credentials for authentication
- Receives print jobs via WebSocket commands
- Reports printer status and job progress
- Downloads files using one-time tokens

### Third-party Applications
**Integration:**
- OAuth2 client credentials flow
- Scope-based permissions (`print:submit`)
- REST API for job submission and status polling

## Error Handling

**Strategy:** Centralized error response with standardized format

**Patterns:**
- Handler returns HTTP status code + JSON response
- Repository returns wrapped errors with context
- WebSocket errors logged, connection closed on fatal errors
- Background task errors logged, task continues

**Response Format:**
```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

## Cross-Cutting Concerns

**Logging:** Zap structured logging throughout (`api/internal/logger/`)

**Validation:** 
- Go: go-playground/validator for struct validation
- Security validation: username, email, password strength

**Authentication:**
- JWT for session management
- OAuth2 scope-based access control
- One-time tokens for file access

**Rate Limiting:** Nginx + ulule/limiter (10 req/s per IP)

---

*Architecture analysis: 2025-03-17*
