# Technology Stack

**Analysis Date:** 2026-03-18

## Languages

**Primary:**
- **Go 1.25.0** - Backend API service (`api/cmd/server/main.go`)
- **TypeScript 4.7.4** - Admin frontend React application (`admin/package.json`)

**Secondary:**
- **YAML** - Configuration files (`api/config.example.yaml`, `docker-compose.yml`)
- **Dockerfile** - Container build definitions
- **Nginx Config** - Reverse proxy configuration (`nginx/nginx.conf`, `nginx/conf.d/admin.conf`)

## Runtime

**Backend:**
- Go 1.25.0 (Alpine Linux in Docker)
- Runtime: Native Go runtime with CGO disabled

**Frontend:**
- Node.js 18 (Alpine Linux in Docker)
- Browser target: ES5 with modern browser support

**Package Manager:**
- Go Modules (go.mod/go.sum present)
- npm 8.x (package-lock.json present)

## Frameworks

**Backend:**
- **Gin Web Framework v1.12.0** - HTTP web framework for REST API (`github.com/gin-gonic/gin`)
- **Swagger/OpenAPI** - API documentation via `swaggo` suite
  - `github.com/swaggo/gin-swagger v1.6.1`
  - `github.com/swaggo/swag v1.16.6`

**Frontend:**
- **React 18.2.0** - UI framework (`admin/package.json`)
- **React Router 6.20.1** - Client-side routing
- **Ant Design 5.12.8** - UI component library
- **ECharts 5.4.3** - Data visualization/charts
- **Create React App 5.0.1** - Build toolchain

**Testing:**
- Jest (via React Testing Library)
- React Testing Library (`@testing-library/react` v13.3.0)

## Key Dependencies

**Backend - Core:**
| Package | Version | Purpose |
|---------|---------|---------|
| gin-gonic/gin | v1.12.0 | HTTP web framework |
| golang-jwt/jwt | v5.2.0 | JWT authentication |
| lib/pq | v1.10.9 | PostgreSQL driver |
| gorilla/websocket | v1.5.1 | WebSocket connections for edge nodes |
| pdfcpu/pdfcpu | v0.11.1 | PDF processing and validation |
| spf13/viper | v1.18.2 | Configuration management |
| go.uber.org/zap | v1.26.0 | Structured logging |
| golang.org/x/crypto | v0.48.0 | Cryptographic operations (bcrypt) |
| golang.org/x/oauth2 | v0.15.0 | OAuth2 client for Keycloak integration |
| ulule/limiter | v3.11.2 | Rate limiting |
| go-playground/validator | v10.30.1 | Request validation |
| google/uuid | v1.5.0 | UUID generation |

**Frontend - Core:**
| Package | Version | Purpose |
|---------|---------|---------|
| react | 18.2.0 | UI framework |
| react-dom | 18.2.0 | DOM renderer |
| react-router-dom | 6.20.1 | Routing |
| antd | 5.12.8 | UI components |
| @ant-design/icons | 5.2.6 | Icon library |
| echarts | 5.4.3 | Charts/visualization |
| typescript | 4.7.4 | Type checking |

## Configuration

**Environment Configuration:**
- Primary: Environment variables with `FLY_PRINT_` prefix (via Viper)
- Secondary: YAML config file (`config.yaml`) in multiple search paths:
  - Current working directory
  - `./configs/`
  - `/etc/fly-print-cloud/`

**Key Configuration Files:**
- `api/config.example.yaml` - Template for API configuration
- `.env.example` - Docker Compose environment template
- `admin/.env.example` - Frontend build environment template

**Frontend Build-time Config:**
- `REACT_APP_API_BASE_PATH` - API base path (default: `/api/v1`)
- `REACT_APP_AUTH_BASE_PATH` - Auth base path (default: `/auth`)
- `REACT_APP_API_URL` - Full API URL (legacy compatibility)

## Infrastructure

**Containerization:**
- **Docker** - Multi-stage builds for both API and Admin
- **Docker Compose** - Orchestrates 4 services:
  - `postgres` - PostgreSQL 15 database
  - `api` - Go backend API
  - `admin-console-builder` - React build process
  - `nginx` - Reverse proxy and static file server

**Reverse Proxy:**
- **Nginx** - Handles routing, rate limiting, WebSocket upgrades
- Rate limit: 10 requests/second per IP (burst: 20)
- Max body size: 20MB (for file uploads)

**Database:**
- **PostgreSQL 15** - Primary data store
- Connection via `lib/pq` driver
- Connection pooling: 25 max open, 5 max idle, 5min max lifetime

## Platform Requirements

**Development:**
- Go 1.25.0+
- Node.js 18+
- PostgreSQL 15+
- Docker & Docker Compose (optional)

**Production:**
- Docker-compatible container runtime
- Linux host (Alpine-based images)
- Port 8080 for API (internal)
- Port 80/443 for Nginx (external)
- Persistent volumes for:
  - PostgreSQL data (`postgres_data`)
  - File uploads (`file_uploads`)
  - Admin build artifacts (`admin_build`)

## Build Configuration

**Backend Build:**
```dockerfile
# Multi-stage build
FROM golang:1.25-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server
FROM alpine:latest
EXPOSE 8080
```

**Frontend Build:**
```dockerfile
# Multi-stage build
FROM node:18-alpine AS builder
ENV npm_config_registry=https://registry.npmmirror.com
RUN npm install && npm run build
FROM alpine:latest
COPY --from=builder /app/build /app/dist
```

**TypeScript Configuration:**
- Target: ES5
- Module: ESNext
- Strict mode: disabled
- JSX: react-jsx

---

*Stack analysis: 2026-03-18*
