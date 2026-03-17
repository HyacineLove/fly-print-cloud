# Technology Stack

**Analysis Date:** 2026-03-17

## Languages

**Primary:**
- **Go 1.25** - Backend API and WebSocket server (`api/`)
- **TypeScript 4.7.4** - Admin console frontend (`admin/src/`)

**Secondary:**
- **SQL** - PostgreSQL schema and queries (`api/internal/database/`)
- **YAML** - Configuration files (`api/config.example.yaml`, `docker-compose.yml`)
- **HTML/CSS** - Frontend markup and styling (via React)

## Runtime

**Environment:**
- **Docker** - Container runtime (Docker Compose deployment)
- **Linux** - Target OS for containers (Alpine Linux)

**Package Manager:**
- **Go Modules** - Go dependency management (`api/go.mod`, `api/go.sum`)
- **npm** - Node.js package manager (`admin/package-lock.json` implied)

## Frameworks

**Backend (Go):**
- **Gin v1.12.0** - HTTP web framework (`api/internal/handlers/`, `api/cmd/server/main.go`)
- **Gorilla WebSocket v1.5.1** - WebSocket implementation (`api/internal/websocket/`)
- **Viper v1.18.2** - Configuration management (`api/internal/config/config.go`)

**Frontend (TypeScript/React):**
- **React 18.2.0** - UI framework (`admin/src/index.tsx`)
- **React Router v6.20.1** - Client-side routing (`admin/src/App.tsx`)
- **Ant Design v5.12.8** - UI component library (inferred from `package.json`)
- **ECharts v5.4.3** - Data visualization library (`package.json`)

**Testing:**
- **Jest** (via `react-scripts`) - JavaScript testing framework
- **React Testing Library** - Component testing utilities

**Build/Dev:**
- **Create React App 5.0.1** - React build toolchain (`admin/package.json`)
- **Go build** - Native Go compilation (`api/Dockerfile`)
- **Docker Compose** - Multi-container orchestration (`docker-compose.yml`)
- **Nginx** - Reverse proxy and static file server (`nginx/`, `nginx/nginx.conf`)

## Key Dependencies

**Backend Critical:**
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | v1.12.0 | HTTP web framework |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/gorilla/websocket` | v1.5.1 | WebSocket connections |
| `github.com/golang-jwt/jwt/v5` | v5.2.0 | JWT token handling |
| `github.com/pdfcpu/pdfcpu` | v0.11.1 | PDF processing |
| `github.com/ulule/limiter/v3` | v3.11.2 | Rate limiting |
| `go.uber.org/zap` | v1.26.0 | Structured logging |
| `golang.org/x/crypto` | v0.48.0 | Password hashing (bcrypt) |
| `golang.org/x/oauth2` | v0.15.0 | OAuth2 client support |
| `github.com/go-playground/validator/v10` | v10.30.1 | Input validation |
| `github.com/swaggo/gin-swagger` | v1.6.1 | API documentation |
| `github.com/spf13/viper` | v1.18.2 | Configuration management |

**Frontend Critical:**
| Package | Version | Purpose |
|---------|---------|---------|
| `react` | ^18.2.0 | UI framework |
| `react-dom` | ^18.2.0 | DOM renderer |
| `react-router-dom` | ^6.20.1 | Routing |
| `antd` | ^5.12.8 | UI components |
| `@ant-design/icons` | ^5.2.6 | Icons |
| `echarts` | ^5.4.3 | Charts/visualization |
| `typescript` | ^4.7.4 | Type checking |

## Configuration

**Environment:**
- **Viper** - YAML + environment variable configuration (`api/internal/config/config.go`)
- **Environment variables** - `FLY_PRINT_*` prefix for all settings
- **`.env` file** - Docker Compose environment (`docker-compose.yml`)

**Key Configuration Files:**
- `api/config.example.yaml` - API configuration template
- `.env.example` - Docker Compose environment template
- `docker-compose.yml` - Service orchestration
- `nginx/nginx.conf` - Reverse proxy configuration
- `nginx/conf.d/admin.conf` - Nginx site configuration

**Build:**
- `api/Dockerfile` - Go API container (multi-stage build)
- `admin/Dockerfile` - React admin console container (multi-stage build)

## Platform Requirements

**Development:**
- Go 1.25+ (for API development)
- Node.js 18+ (for frontend development)
- Docker & Docker Compose (for full stack)
- PostgreSQL 15+ (or use Docker)

**Production:**
- Docker runtime environment
- PostgreSQL 15 database
- Nginx reverse proxy
- Linux-based containers (Alpine)

## Infrastructure Services

**Container Images:**
- `docker.m.daocloud.io/library/postgres:15` - Database
- `docker.m.daocloud.io/library/golang:1.25-alpine` - Go build environment
- `docker.m.daocloud.io/library/node:18-alpine` - Node build environment
- `docker.m.daocloud.io/library/alpine:latest` - Runtime base image
- `docker.m.daocloud.io/library/nginx:alpine` - Reverse proxy

---

*Stack analysis: 2026-03-17*
