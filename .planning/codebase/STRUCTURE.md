# Codebase Structure

**Analysis Date:** 2025-03-17

## Directory Layout

```
fly-print-cloud/
├── api/                          # Go Backend API
│   ├── cmd/
│   │   └── server/
│   │       └── main.go           # Application entry point
│   ├── internal/
│   │   ├── auth/                 # Authentication services
│   │   ├── config/               # Configuration management
│   │   ├── database/             # Database repositories
│   │   ├── handlers/             # HTTP request handlers
│   │   ├── logger/               # Structured logging
│   │   ├── middleware/           # HTTP middleware
│   │   ├── models/               # Domain models
│   │   ├── security/             # Security utilities
│   │   ├── utils/                # Utility functions
│   │   └── websocket/            # WebSocket handling
│   ├── docs/                     # Swagger-generated docs
│   ├── go.mod                    # Go module definition
│   ├── go.sum                    # Go dependency checksums
│   ├── Dockerfile                # API container build
│   └── config.example.yaml       # Configuration template
│
├── admin/                        # React Admin Console
│   ├── src/
│   │   ├── components/
│   │   │   ├── pages/            # Page components
│   │   │   ├── ErrorBoundary.tsx # Error handling
│   │   │   └── Loading.tsx       # Loading states
│   │   ├── services/             # API client
│   │   ├── utils/                # Utilities
│   │   ├── App.tsx               # Root component
│   │   ├── config.ts             # App configuration
│   │   └── index.tsx             # Entry point
│   ├── public/
│   │   └── index.html            # HTML template
│   ├── package.json              # NPM dependencies
│   ├── tsconfig.json             # TypeScript config
│   └── Dockerfile                # Admin build container
│
├── nginx/                        # Reverse Proxy
│   ├── nginx.conf                # Main Nginx config
│   ├── conf.d/
│   │   └── admin.conf            # Site configuration
│   └── ssl/                      # SSL certificate scripts
│
├── docker-compose.yml            # Full stack orchestration
├── .env.example                  # Environment template
├── .gitignore                    # Git ignore rules
└── README.md                     # Project documentation
```

## Directory Purposes

### `api/` - Backend API
- **Purpose:** Core cloud printing service
- **Language:** Go 1.25
- **Framework:** Gin Web Framework
- **Key files:**
  - `api/cmd/server/main.go` - Application bootstrap
  - `api/internal/handlers/` - HTTP handlers (14 files)
  - `api/internal/database/` - Data access layer (7 files)

### `api/internal/handlers/` - HTTP Handlers
- **Purpose:** Process HTTP requests, delegate to services
- **Contains:** 14 handler files for different domains
- **Key files:**
  - `user_handler.go` - User CRUD
  - `edge_node_handler.go` - Edge node management
  - `printer_handler.go` - Printer operations
  - `print_job_handler.go` - Job lifecycle
  - `oauth2_handler.go` - Authentication flows
  - `file_handler.go` - File operations
  - `response.go` - Response helpers
  - `common.go` - Shared utilities (pagination)

### `api/internal/database/` - Data Access Layer
- **Purpose:** Database operations, SQL queries
- **Contains:** Repository pattern implementations
- **Key files:**
  - `database.go` - Connection management, schema initialization
  - `user_repository.go` - User queries
  - `edge_node_repository.go` - Edge node queries
  - `printer_repository.go` - Printer queries
  - `print_job_repository.go` - Job queries
  - `file_repository.go` - File metadata queries
  - `token_usage_repository.go` - Token tracking

### `api/internal/models/` - Domain Models
- **Purpose:** Business entity definitions
- **Key files:**
  - `models.go` - Core entities (User, Printer, PrintJob, EdgeNode, PrinterCapabilities)
  - `file.go` - File-related models
  - `oauth2_client.go` - OAuth2 client model

### `api/internal/middleware/` - HTTP Middleware
- **Purpose:** Cross-cutting HTTP concerns
- **Key files:**
  - `oauth2.go` - OAuth2 validation (364 lines)
  - `common.go` - CORS, logging, security headers
  - `edge_node_check.go` - Edge node state validation
  - `printer_check.go` - Printer state validation

### `api/internal/websocket/` - Real-time Communication
- **Purpose:** WebSocket server for edge node connections
- **Key files:**
  - `manager.go` - Connection registry (407 lines)
  - `handler.go` - HTTP upgrade handling
  - `connection.go` - Individual connection management
  - `message.go` - Protocol message types
  - `errors.go` - WebSocket errors

### `api/internal/security/` - Security Utilities
- **Purpose:** Token management, validation
- **Key files:**
  - `token_manager.go` - One-time token generation/validation
  - `validation.go` - Input validation helpers

### `api/internal/config/` - Configuration
- **Purpose:** App configuration with Viper
- **Key file:** `config.go` - Config structs, loading, validation (298 lines)

### `admin/` - Admin Console (React)
- **Purpose:** Management UI
- **Framework:** React 18 + TypeScript + Ant Design 5
- **Build Tool:** Create React App (react-scripts 5)

### `admin/src/components/pages/` - Page Components
- **Purpose:** Main application pages
- **Contains:** 10 page components
- **Files:**
  - `Dashboard.tsx` - Analytics dashboard
  - `EdgeNodes.tsx` - Edge node management
  - `Printers.tsx` - Printer management
  - `PrintJobs.tsx` - Job monitoring
  - `Users.tsx` - User administration
  - `OAuth2Clients.tsx` - OAuth2 client management
  - `Settings.tsx` - System settings
  - `PublicUpload.tsx` - Public file upload
  - `Login.tsx` - Login page

### `admin/src/services/` - API Client
- **Purpose:** HTTP client for API communication
- **Key file:** `api.ts` - Axios-like fetch wrapper with auth

### `nginx/` - Reverse Proxy
- **Purpose:** Static file serving, API proxying, WebSocket support
- **Key files:**
  - `nginx.conf` - Main configuration (34 lines)
  - `conf.d/admin.conf` - Site config with routes (67 lines)

## Key File Locations

### Entry Points
- **Backend:** `api/cmd/server/main.go`
- **Frontend:** `admin/src/index.tsx`
- **WebSocket Handler:** `api/internal/websocket/handler.go`

### Configuration
- **Backend Config:** `api/internal/config/config.go`
- **Config Template:** `api/config.example.yaml`
- **Frontend Config:** `admin/src/config.ts`
- **Docker Compose:** `docker-compose.yml`
- **Env Template:** `.env.example`

### Core Logic
- **Main Router:** `api/cmd/server/main.go` (lines 213-353 in setupRoutes)
- **Database Schema:** `api/internal/database/database.go` (lines 90-441)
- **Auth Middleware:** `api/internal/middleware/oauth2.go`

### Testing
- No test files detected in the codebase
- **Test locations:** Not applicable

## Naming Conventions

### Go Backend

**Files:**
- `*_handler.go` - HTTP request handlers
- `*_repository.go` - Database repositories
- `*_service.go` - Business logic services
- `*_test.go` - Test files (convention, none present)

**Types:**
- `PascalCase` for exported types: `UserHandler`, `PrintJobRepository`
- Constructor pattern: `NewUserHandler()`, `NewPrintJobRepository()`

**Variables:**
- `camelCase` for local variables
- Abbreviations allowed: `db`, `cfg`, `repo`

**Functions:**
- `PascalCase` for exported: `CreateUser()`, `GetPrinterByID()`
- `camelCase` for internal: `parsePaginationParams()`
- Handler methods: `(h *Handler) MethodName()`
- Repository methods: `(r *Repository) MethodName()`

### React Frontend

**Files:**
- `PascalCase.tsx` for components: `Dashboard.tsx`, `UserManagement.tsx`
- `camelCase.ts` for utilities: `api.ts`, `errorHandler.ts`

**Components:**
- Function components with hooks
- Props interfaces defined inline
- Default exports for pages

**Variables:**
- `camelCase` for variables and functions
- `SCREAMING_SNAKE_CASE` for constants

## Where to Add New Code

### New API Endpoint
1. **Handler:** Add method to existing handler in `api/internal/handlers/`
2. **Routes:** Register in `api/cmd/server/main.go` `setupRoutes()` function
3. **Model:** Add to `api/internal/models/` if new entity
4. **Repository:** Add to `api/internal/database/` if database operation needed

### New Entity (Full CRUD)
1. **Model:** Create `api/internal/models/{entity}.go`
2. **Repository:** Create `api/internal/database/{entity}_repository.go`
3. **Handler:** Create `api/internal/handlers/{entity}_handler.go`
4. **Routes:** Register in `api/cmd/server/main.go`
5. **Migration:** Add table creation in `api/internal/database/database.go` `InitTables()`

### New WebSocket Message Type
1. **Protocol:** Add to `api/internal/websocket/message.go`
2. **Handler:** Add case in `api/internal/websocket/connection.go` `readLoop()`
3. **Dispatcher:** Add method in `api/internal/websocket/manager.go`

### New Admin Page
1. **Component:** Create `admin/src/components/pages/{PageName}.tsx`
2. **Route:** Add to `admin/src/App.tsx` routes
3. **Menu:** Add to menu items in `AdminApp` component
4. **API:** Add endpoint calls in `admin/src/services/api.ts` if needed

### New Middleware
1. **Implementation:** Create in `api/internal/middleware/`
2. **Registration:** Add to `api/cmd/server/main.go` middleware chain

### Background Task
1. **Implementation:** Add function in `api/cmd/server/main.go`
2. **Startup:** Call with `go start{TaskName}Task(...)` in `main()`

## Special Directories

### `api/internal/`
- **Purpose:** Private application code (Go convention)
- **Access:** Cannot be imported by external packages
- **Contains:** All business logic

### `api/docs/`
- **Purpose:** Swagger-generated API documentation
- **Generated:** Yes (via swaggo)
- **Committed:** Yes

### `admin/build/` (generated)
- **Purpose:** Production build output
- **Generated:** Yes (via `npm run build`)
- **Committed:** No (in .gitignore)
- **Served:** By Nginx from volume mount

### `nginx/ssl/`
- **Purpose:** SSL certificate generation scripts
- **Contains:** `generate_certs.sh`, `generate_certs.ps1`
- **Note:** SSL currently disabled in docker-compose

---

*Structure analysis: 2025-03-17*
