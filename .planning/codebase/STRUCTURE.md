# Codebase Structure

**Analysis Date:** 2026-03-18

## Directory Layout

```
fly-print-cloud/
├── api/                          # Go backend API
│   ├── cmd/server/              # Application entry point
│   │   └── main.go              # Server bootstrap
│   ├── internal/                # Private application code
│   │   ├── auth/                # Authentication services
│   │   ├── config/              # Configuration management
│   │   ├── database/            # Repository layer
│   │   ├── handlers/            # HTTP handlers
│   │   ├── logger/              # Logging utilities
│   │   ├── middleware/          # HTTP middleware
│   │   ├── models/              # Domain models
│   │   ├── security/            # Security utilities
│   │   ├── utils/               # General utilities
│   │   └── websocket/           # WebSocket handling
│   ├── docs/                    # Swagger documentation
│   ├── Dockerfile               # API container image
│   ├── go.mod                   # Go module definition
│   └── go.sum                   # Dependency checksums
│
├── admin/                        # React admin console
│   ├── src/
│   │   ├── components/          # React components
│   │   │   ├── pages/           # Page components
│   │   │   ├── ErrorBoundary.tsx
│   │   │   └── Loading.tsx
│   │   ├── services/            # API client
│   │   ├── utils/               # Utilities
│   │   ├── App.tsx              # Main app component
│   │   ├── config.ts            # Frontend configuration
│   │   └── index.tsx            # Entry point
│   ├── package.json             # NPM dependencies
│   ├── tsconfig.json            # TypeScript config
│   └── Dockerfile               # Admin build container
│
├── nginx/                        # Nginx configuration
│   ├── conf.d/                  # Site configurations
│   │   └── admin.conf           # Main site config
│   ├── ssl/                     # SSL certificates (if enabled)
│   └── nginx.conf               # Main nginx config
│
├── docker-compose.yml           # Service orchestration
├── .env.example                 # Environment template
├── .env                         # Local environment (gitignored)
└── .planning/                   # Planning documents
    └── codebase/                # Architecture docs
```

## Directory Purposes

**`api/cmd/server/`:**
- Purpose: Application entry point
- Contains: `main.go` - server initialization and dependency injection
- Key files: `api/cmd/server/main.go`

**`api/internal/handlers/`:**
- Purpose: HTTP request handlers
- Contains: One handler per domain entity
- Key files:
  - `api/internal/handlers/printer_handler.go` (604 lines)
  - `api/internal/handlers/print_job_handler.go`
  - `api/internal/handlers/edge_node_handler.go`
  - `api/internal/handlers/oauth2_handler.go`
  - `api/internal/handlers/file_handler.go`

**`api/internal/database/`:**
- Purpose: Data access layer
- Contains: Repository implementations for each entity
- Key files:
  - `api/internal/database/database.go` (559 lines) - DB connection and schema
  - `api/internal/database/printer_repository.go` (448 lines)
  - `api/internal/database/print_job_repository.go`
  - `api/internal/database/edge_node_repository.go`

**`api/internal/models/`:**
- Purpose: Domain entity definitions
- Contains: Structs for database entities
- Key files: `api/internal/models/models.go` (136 lines)

**`api/internal/websocket/`:**
- Purpose: Real-time communication
- Contains: Connection management and message handling
- Key files:
  - `api/internal/websocket/manager.go` (407 lines)
  - `api/internal/websocket/connection.go`
  - `api/internal/websocket/handler.go`

**`api/internal/middleware/`:**
- Purpose: Cross-cutting HTTP concerns
- Contains: Auth, CORS, security headers
- Key files: `api/internal/middleware/oauth2.go`, `common.go`

**`admin/src/components/pages/`:**
- Purpose: Admin console page components
- Contains: One component per page/route
- Key files:
  - `admin/src/components/pages/Dashboard.tsx`
  - `admin/src/components/pages/Printers.tsx`
  - `admin/src/components/pages/PrintJobs.tsx`
  - `admin/src/components/pages/EdgeNodes.tsx`

**`nginx/conf.d/`:**
- Purpose: Nginx site configuration
- Contains: Reverse proxy rules for API and WebSocket
- Key files: `nginx/conf.d/admin.conf`

## Key File Locations

**Entry Points:**
- `api/cmd/server/main.go` - Go API server
- `admin/src/index.tsx` - React application

**Configuration:**
- `api/internal/config/config.go` - Go config (Viper-based)
- `admin/src/config.ts` - Frontend config (env-based)
- `docker-compose.yml` - Service orchestration
- `nginx/nginx.conf` - Web server config

**Core Logic:**
- `api/internal/handlers/*.go` - API endpoints
- `api/internal/websocket/manager.go` - Real-time communication
- `api/internal/security/token_manager.go` - Token generation

**Testing:**
- No test files detected in current codebase

## Naming Conventions

**Go Files:**
- Pattern: `snake_case.go` for implementation files
- Pattern: `*_test.go` for test files (not present)
- Example: `printer_handler.go`, `printer_repository.go`

**TypeScript/TSX Files:**
- Pattern: `PascalCase.tsx` for components
- Pattern: `camelCase.ts` for utilities
- Example: `Printers.tsx`, `api.ts`, `config.ts`

**Directories:**
- Pattern: `snake_case` for Go packages
- Example: `edge_node_handler.go` is in `handlers/` package

## Where to Add New Code

**New API Endpoint:**
1. Add handler method to appropriate `api/internal/handlers/*_handler.go`
2. Register route in `api/cmd/server/main.go` (setupRoutes function)
3. Add model to `api/internal/models/` if new entity
4. Add repository methods to `api/internal/database/*_repository.go`

**New Repository:**
1. Create `api/internal/database/<entity>_repository.go`
2. Follow pattern: struct wrapping `*DB`, constructor `New<Entity>Repository`
3. Implement CRUD methods returning concrete types
4. Wire up in `api/cmd/server/main.go`

**New Frontend Page:**
1. Create `admin/src/components/pages/<PageName>.tsx`
2. Add route in `admin/src/App.tsx`
3. Add menu item in `AdminApp` component
4. Add API calls to `admin/src/services/api.ts` if needed

**New Middleware:**
1. Create middleware function in `api/internal/middleware/`
2. Follow Gin middleware signature: `func() gin.HandlerFunc`
3. Apply in `api/cmd/server/main.go` route setup

## Special Directories

**`api/internal/`:**
- Purpose: Private application code (Go convention)
- Importable only within `fly-print-cloud/api` module
- Not exposed to external packages

**`api/docs/`:**
- Purpose: Swagger/OpenAPI documentation
- Generated by swaggo (auto-generated, do not edit manually)
- Committed: Yes (for API documentation availability)

**`nginx/ssl/`:**
- Purpose: SSL certificates (currently disabled)
- Generated: No (manual placement)
- Committed: No (gitignored)

**`.planning/codebase/`:**
- Purpose: Architecture documentation
- Generated: No (manual)
- Committed: Yes

## File Size Reference

Largest files (indicating complexity):
1. `api/cmd/server/main.go` - 532 lines (bootstrap)
2. `api/internal/database/database.go` - 559 lines (schema + migrations)
3. `api/internal/handlers/printer_handler.go` - 604 lines
4. `api/internal/websocket/manager.go` - 407 lines
5. `api/internal/config/config.go` - 298 lines
6. `admin/src/App.tsx` - 280 lines
7. `admin/src/services/api.ts` - 238 lines

---

*Structure analysis: 2026-03-18*
