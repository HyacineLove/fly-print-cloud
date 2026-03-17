# External Integrations

**Analysis Date:** 2026-03-17

## APIs & External Services

**Authentication Providers:**
- **Built-in OAuth2** - Custom implementation (`api/internal/auth/builtin_auth_service.go`)
  - Supports Client Credentials Flow (for Edge nodes)
  - Supports Password Grant Flow (for admin users)
  - JWT tokens with HMAC-SHA256 signing
  - Configurable via `oauth2.mode=builtin`

- **Keycloak** (optional) - External identity provider
  - Authorization Code Flow
  - Token validation via UserInfo endpoint
  - Configurable via `oauth2.mode=keycloak`
  - Endpoints: auth, token, userinfo, logout, JWKS

**WebSocket Communication:**
- **Gorilla WebSocket** - Real-time Edge node communication
  - Endpoint: `/api/v1/edge/ws`
  - Authentication: OAuth2 Bearer token required
  - Scope required: `edge:heartbeat`
  - Connection limit: 1000 concurrent connections
  - Message size limit: 10MB

## Data Storage

**Primary Database:**
- **PostgreSQL 15** - Main relational database
  - Driver: `github.com/lib/pq` v1.10.9
  - Connection pooling (max 25 open, 5 idle)
  - Connection lifetime: 5 minutes
  - Tables: users, edge_nodes, printers, print_jobs, files, oauth2_clients, token_usage_records

**File Storage:**
- **Local filesystem** - Uploaded files stored in container
  - Path: `/root/uploads` (configurable via `storage.upload_dir`)
  - Max file size: 10MB (configurable)
  - Max document pages: 5 (PDF/DOCX validation)
  - Cleanup task: Deletes files older than 24 hours

**Caching:**
- **In-memory rate limiting** - Token bucket algorithm
  - Store: Memory-based (`github.com/ulule/limiter/v3`)
  - Rate: 10 requests/second per IP
  - Burst: 20 requests

## Authentication & Identity

**OAuth2 Scopes:**
| Scope | Purpose |
|-------|---------|
| `fly-print-admin` | Full admin access |
| `fly-print-operator` | Operator-level access |
| `edge:register` | Edge node registration |
| `edge:printer` | Printer management by Edge nodes |
| `edge:heartbeat` | WebSocket connection and heartbeat |
| `file:read` | File download access |
| `print:submit` | Submit print jobs via API |

**JWT Configuration:**
- Algorithm: HS256 (HMAC-SHA256)
- Token expiry: 3600 seconds (1 hour, configurable)
- Issuer: `fly-print-cloud` (configurable)
- Claims: sub, preferred_username, email, scope, realm_access.roles

**Password Security:**
- Algorithm: bcrypt
- Cost: bcrypt.DefaultCost

## Webhooks & Callbacks

**Incoming:**
- **OAuth2 Callback** - `/auth/callback`
  - Handles Keycloak authorization code exchange
  - Redirects to admin console after authentication

**Outgoing:**
- None (system operates on pull model via WebSocket)

## Third-Party Services

**Document Processing:**
- **pdfcpu v0.11.1** - PDF validation and page counting
  - Used for preflight document validation
  - Enforces max page count limits

**Monitoring & Observability:**
- **Zap** - Structured JSON logging (`go.uber.org/zap`)
  - Debug/production modes
  - Log rotation via Docker (json-file driver, 50MB max, 5 files)

## Infrastructure Services

**Reverse Proxy:**
- **Nginx** - Traffic routing and static files
  - Routes: `/api/*` → API service
  - Routes: `/auth/*` → API service (OAuth2)
  - Routes: `/api/v1/edge/ws` → WebSocket upgrade
  - Routes: `/*` → Static admin console
  - Rate limiting: 10 req/s per IP
  - Client max body size: 20MB
  - WebSocket support: Upgrade headers, 86400s timeouts

**Container Services:**
| Service | Image | Purpose |
|---------|-------|---------|
| postgres | postgres:15 | Database server |
| api | Custom Go build | REST API + WebSocket server |
| admin-console-builder | Custom Node build | React build static files |
| nginx | nginx:alpine | Reverse proxy + static server |

**Volumes:**
- `postgres_data` - PostgreSQL persistent storage
- `admin_build` - React build artifacts (shared)
- `file_uploads` - Uploaded file storage

**Networks:**
- `fly-print-network` - Bridge network for inter-service communication

## Environment Configuration

**Required Environment Variables:**
```bash
# Database
POSTGRES_DB=fly_print_cloud
POSTGRES_USER=postgres
POSTGRES_PASSWORD=<secure-password>

# OAuth2 (builtin mode)
OAUTH2_JWT_SIGNING_SECRET=<32+ char secret>

# Security
FILE_ACCESS_SECRET=<32+ char secret>

# CORS
ALLOWED_ORIGINS=http://localhost:8012,...
```

**Configuration Sources (Priority high→low):**
1. Environment variables (`FLY_PRINT_*`)
2. `config.yaml` file (if present)
3. Default values in code

**Secrets Management:**
- Environment variables via `.env` file (not committed)
- Docker secrets (optional, via environment)
- bcrypt-hashed passwords in database
- HMAC-signed JWT tokens
- One-time tokens for file upload/download (stored in DB)

## API Documentation

**Swagger/OpenAPI:**
- Generated documentation at `/swagger/*`
- Middleware: `github.com/swaggo/gin-swagger`
- Annotations in handler files (`api/internal/handlers/`)

---

*Integration audit: 2026-03-17*
