# External Integrations

**Analysis Date:** 2026-03-18

## APIs & External Services

**Authentication Providers:**

1. **Built-in OAuth2/JWT (Default)**
   - Implementation: Custom JWT service (`api/internal/auth/builtin_auth_service.go`)
   - Library: `github.com/golang-jwt/jwt/v5`
   - Modes supported:
     - `builtin` - Embedded authentication (default for LAN/campus deployment)
     - `keycloak` - External Keycloak identity provider
   - Token expiry: Configurable (default 3600 seconds)
   - Scopes: `admin:*`, `fly-print-operator`, `print:submit`, `edge:*`, `file:read`

2. **Keycloak (Optional)**
   - Protocol: OpenID Connect / OAuth2
   - Configuration endpoints:
     - Auth URL: `OAUTH2_AUTH_URL`
     - Token URL: `OAUTH2_TOKEN_URL`
     - UserInfo URL: `OAUTH2_USERINFO_URL`
     - Logout URL: `OAUTH2_LOGOUT_URL`
   - Required env vars:
     - `OAUTH2_CLIENT_ID`
     - `OAUTH2_CLIENT_SECRET`
     - `OAUTH2_REDIRECT_URI`

**Edge Node Communication:**
- **WebSocket** - Real-time bidirectional communication
  - Endpoint: `/api/v1/edge/ws`
  - Library: `github.com/gorilla/websocket`
  - Purpose: Printer status updates, job dispatching, heartbeats
  - Nginx proxy timeout: 86400 seconds (24 hours)

**Rate Limiting:**
- Implementation: `github.com/ulule/limiter/v3`
- Storage: In-memory
- Rate: 10 requests/second per IP
- Burst: 20 requests
- Nginx additional layer: Same 10r/s with burst 20

## Data Storage

**Primary Database:**
- **PostgreSQL 15**
  - Driver: `github.com/lib/pq`
  - Connection: Via environment variables or config.yaml
  - Key env vars:
    - `FLY_PRINT_DATABASE_HOST`
    - `FLY_PRINT_DATABASE_PORT` (default: 5432)
    - `FLY_PRINT_DATABASE_DBNAME`
    - `FLY_PRINT_DATABASE_USER`
    - `FLY_PRINT_DATABASE_PASSWORD`
    - `FLY_PRINT_DATABASE_SSLMODE` (default: disable)

**Database Schema:**
| Table | Purpose |
|-------|---------|
| `users` | Admin/operator user accounts |
| `edge_nodes` | Edge node registrations |
| `printers` | Printer configurations |
| `print_jobs` | Print job queue and history |
| `files` | Uploaded file metadata |
| `oauth2_clients` | OAuth2 client credentials (builtin mode) |
| `token_usage_records` | One-time token tracking |

**File Storage:**
- Location: Local filesystem (`/root/uploads` in container)
- Configurable path: `storage.upload_dir` (default: `./uploads`)
- Max file size: 10MB (configurable, default 10485760 bytes)
- Max document pages: 5 (PDF/DOCX validation via pdfcpu)
- Cleanup: Automated daily deletion of files older than 24 hours

**No External Cache:**
- Rate limiting uses in-memory store
- No Redis or external cache detected

## Authentication & Identity

**Two Authentication Modes:**

1. **Builtin Mode (Default)**
   - JWT signing: HMAC SHA256
   - Secret: `FLY_PRINT_OAUTH2_JWT_SIGNING_SECRET` (min 32 chars)
   - Password hashing: bcrypt with default cost
   - Token transport: HTTP-only cookies + Bearer tokens
   - User storage: PostgreSQL `users` table

2. **Keycloak Mode**
   - External identity provider
   - User sync: On first login, creates local user record
   - Token validation: Via UserInfo endpoint or JWKS
   - Logout: RP-initiated logout with redirect

**Authorization Scopes:**
| Scope | Description |
|-------|-------------|
| `fly-print-admin` | Full admin access |
| `fly-print-operator` | Operator (limited admin) |
| `print:submit` | Submit print jobs (third-party API) |
| `edge:register` | Register edge nodes |
| `edge:printer` | Manage printers on edge nodes |
| `file:read` | Download files |

## Security Features

**File Access Security:**
- Signed tokens for upload/download (HMAC SHA256)
- Secret: `FLY_PRINT_SECURITY_FILE_ACCESS_SECRET`
- Token TTL: Configurable (upload: 300s default, download: 300s default)
- One-time use tokens tracked in database

**CORS Configuration:**
- Origins: Configurable via `FLY_PRINT_SERVER_ALLOWED_ORIGINS`
- Default includes common development URLs
- Supports wildcard patterns for LAN IPs (e.g., `http://192.168.*.*`)

**Security Headers (Nginx):**
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`

## Monitoring & Observability

**Logging:**
- Framework: `go.uber.org/zap`
- Structured JSON logging
- Levels: Debug, Info, Warn, Error, Fatal
- Log rotation: Docker json-file driver (50MB max, 5 files)

**Health Checks:**
- Basic: `GET /health` (no auth required)
- Detailed: `GET /api/v1/health` (auth required)
- Database health: Included in detailed check
- WebSocket manager: Included in detailed check

**No External Monitoring:**
- No Prometheus, DataDog, or similar detected
- No error tracking service (Sentry, etc.)

## CI/CD & Deployment

**Container Registry:**
- Base images from `docker.m.daocloud.io` (mirror of Docker Hub)
- Alternative: `dockerpull.pw` (commented out)

**Build Process:**
1. Go API: Multi-stage Docker build
2. React Admin: Multi-stage build with static export
3. Nginx: Serves static files and proxies API requests

**Deployment Model:**
- Docker Compose for single-host deployment
- Services: postgres, api, admin-console-builder, nginx
- No Kubernetes manifests detected

## Webhooks & Callbacks

**OAuth2 Callbacks:**
- Endpoint: `GET /auth/callback`
- Used by: Keycloak mode only
- Redirect after: Token exchange and cookie setting

**Nginx Auth Request:**
- Endpoint: `GET /auth/verify`
- Used by: Nginx `auth_request` (if configured)
- Returns: 200 if authenticated, 401 if not

**No External Webhooks:**
- No outgoing webhooks to external services
- No Slack, Discord, or notification integrations
- No external metrics exporters

## Environment Configuration

**Required Environment Variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `POSTGRES_DB` | Database name | `fly_print_cloud` |
| `POSTGRES_USER` | Database user | `postgres` |
| `POSTGRES_PASSWORD` | Database password | (generate with openssl) |
| `OAUTH2_JWT_SIGNING_SECRET` | JWT signing key (min 32 chars) | (random hex) |
| `FILE_ACCESS_SECRET` | File token signing key (min 32 chars) | (random hex) |
| `ALLOWED_ORIGINS` | CORS allowed origins | comma-separated URLs |

**Optional for Keycloak Mode:**
- `OAUTH2_CLIENT_ID`
- `OAUTH2_CLIENT_SECRET`
- `OAUTH2_AUTH_URL`
- `OAUTH2_TOKEN_URL`
- `OAUTH2_USERINFO_URL`
- `OAUTH2_LOGOUT_URL`

**Secrets Location:**
- Environment variables only (no secret files)
- `.env` file in project root (Docker Compose)
- Never committed: `.env` is in `.gitignore`

## Network Architecture

**Service Communication:**
```
Client → Nginx (port 80/8012)
  ├── /api/* → API (port 8080)
  ├── /auth/* → API (port 8080)
  ├── /api/v1/edge/ws → API (WebSocket upgrade)
  └── /* → Static files (Admin Console)
```

**Internal Network:**
- Docker network: `fly-print-network`
- Inter-service DNS: `postgres`, `api`
- No external API calls except Keycloak (if configured)

## Scaling Considerations

**Current Limitations:**
- Single PostgreSQL instance (no replication)
- In-memory rate limiting (not shared across instances)
- File storage on local filesystem (not distributed)
- WebSocket connections tied to single API instance

**Horizontal Scaling Blockers:**
- WebSocket state not shared
- Rate limiter memory-only
- File uploads local to container

---

*Integration audit: 2026-03-18*
