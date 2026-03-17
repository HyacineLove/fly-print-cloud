# Coding Conventions

**Analysis Date:** 2026-03-17

## Overview

This codebase consists of a Go backend API (`api/`) and a React TypeScript frontend admin panel (`admin/`). Both follow distinct conventions appropriate to their ecosystems.

---

## Go Backend Conventions

### File Organization

**Package Structure:**
- `cmd/server/` - Application entry point (`main.go`)
- `internal/handlers/` - HTTP request handlers (Gin framework)
- `internal/database/` - Repository pattern data access layer
- `internal/models/` - Domain models/structs
- `internal/middleware/` - HTTP middleware (auth, CORS, logging)
- `internal/config/` - Configuration management (Viper-based)
- `internal/auth/` - Authentication services
- `internal/security/` - Security utilities (token management, validation)
- `internal/logger/` - Structured logging (Zap)
- `internal/websocket/` - WebSocket connection management
- `internal/utils/` - Shared utilities (document validators)

### Naming Conventions

**Files:**
- Snake_case for multi-word files: `user_handler.go`, `print_job_repository.go`
- Singular nouns preferred: `handler.go` not `handlers.go` (except package level)

**Types/Structs:**
- PascalCase for exported types: `UserHandler`, `PrintJobRepository`
- Acronyms in uppercase: `OAuth2Config`, `APIResponse`

```go
// From api/internal/handlers/user_handler.go
type UserHandler struct {
    userRepo *database.UserRepository
}

// From api/internal/models/models.go
type PrintJob struct {
    ID         string    `json:"id"`
    Status     string    `json:"status"`
}
```

**Functions:**
- PascalCase for exported functions: `NewUserHandler()`, `GetUserByID()`
- camelCase for unexported functions: `splitTrim()`, `matchWildcard()`
- Constructor pattern: `NewXxx()` for creating instances
- Getter pattern: `GetXxxByYyy()` for retrieval methods

**Variables:**
- camelCase for local variables: `userRepo`, `tokenManager`
- ALL_CAPS for constants (only error codes currently): `ErrCodeUserNotFound`

### Code Style

**Imports Organization:**
1. Standard library imports
2. Internal project imports (with module path)
3. Third-party imports (grouped by purpose)

```go
// From api/cmd/server/main.go
import (
    "context"
    "fmt"
    "net/http"
    
    "fly-print-cloud/api/internal/auth"
    "fly-print-cloud/api/internal/config"
    "fly-print-cloud/api/internal/database"
    
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)
```

**Struct Tags:**
- JSON tags use snake_case: `json:"created_at"`
- Mapstructure tags for config: `mapstructure:"allowed_origins"`
- Omit empty fields: `json:"deleted_at,omitempty"`

**Error Handling:**
- Wrapped errors with context: `fmt.Errorf("failed to create user: %w", err)`
- Repository pattern returns `(*Model, error)` or `error`
- Specific error types for domain errors (see `api/internal/handlers/errors.go`)

```go
// From api/internal/database/user_repository.go
func (r *UserRepository) GetUserByID(id string) (*models.User, error) {
    user := &models.User{}
    err := r.db.QueryRow(query, id).Scan(...)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("user not found")
        }
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    return user, nil
}
```

### Error Code System

**File:** `api/internal/handlers/errors.go`

Error codes are organized by domain:
- `1000-1999`: General errors (BadRequest, Unauthorized, etc.)
- `2000-2099`: User-related errors
- `3000-3099`: Edge Node errors
- `4000-4099`: Printer errors
- `5000-5099`: Print Job errors
- `6000-6099`: File errors
- `7000-7099`: OAuth2 errors

```go
const (
    ErrCodeBadRequest     = 1000
    ErrCodeUserNotFound   = 2000
    ErrCodeEdgeNodeOffline = 3005
)
```

### Response Patterns

**File:** `api/internal/handlers/response.go`

Standardized API responses:

```go
// Success response
func SuccessResponse(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, Response{
        Code:    http.StatusOK,
        Message: "success",
        Data:    data,
    })
}

// Error response
func ErrorResponse(c *gin.Context, code int, message string) {
    c.JSON(code, Response{
        Code:    code,
        Message: message,
    })
}
```

**Pagination Pattern:**
```go
// From api/internal/handlers/common.go
func ParsePaginationParams(c *gin.Context) (page, pageSize, offset int)

// Response includes pagination metadata
PaginatedSuccessResponse(c, items, total, page, pageSize)
```

### Handler Pattern

**Constructor Injection:**
```go
// From api/internal/handlers/user_handler.go
type UserHandler struct {
    userRepo *database.UserRepository
}

func NewUserHandler(userRepo *database.UserRepository) *UserHandler {
    return &UserHandler{userRepo: userRepo}
}
```

**Request/Response Structs:**
```go
type CreateUserRequest struct {
    Username string `json:"username" binding:"required,min=3,max=50"`
    Email    string `json:"email" binding:"required,email"`
}
```

### Repository Pattern

**File:** `api/internal/database/*.go`

```go
type UserRepository struct {
    db *DB
}

func NewUserRepository(db *DB) *UserRepository
func (r *UserRepository) CreateUser(user *models.User) error
func (r *UserRepository) GetUserByID(id string) (*models.User, error)
```

### Logging

**Framework:** Uber Zap (`go.uber.org/zap`)

**File:** `api/internal/logger/logger.go`

```go
// Structured logging with fields
logger.Info("User created", zap.String("username", user.Username))
logger.Error("Database query failed", zap.Error(err), zap.String("query", query))
```

### Comments

- Chinese comments for business logic
- English for technical documentation
- Swagger annotations for API documentation:

```go
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users/profile [get]
```

---

## TypeScript/React Frontend Conventions

### File Organization

**Admin Panel Structure:**
- `src/components/pages/` - Page components (Dashboard, Users, etc.)
- `src/components/` - Shared components (ErrorBoundary, Loading)
- `src/services/` - API service layer (`api.ts`)
- `src/utils/` - Utilities (errorHandler.ts)
- `src/config.ts` - Configuration and URL builders

### Naming Conventions

**Files:**
- PascalCase for components: `Dashboard.tsx`, `ErrorBoundary.tsx`
- camelCase for utilities/services: `errorHandler.ts`, `api.ts`

**Components:**
- PascalCase for component names and their props interfaces

```typescript
// From admin/src/components/ErrorBoundary.tsx
interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
}

class ErrorBoundary extends Component<Props, State>
```

**Interfaces:**
```typescript
// From admin/src/components/pages/Dashboard.tsx
interface DashboardStats {
  totalPrinters: number;
  onlinePrinters: number;
}

interface PrinterStatus {
  id: string;
  name: string;
  status: 'ready' | 'printing' | 'error' | 'offline';
}
```

### Code Style

**Imports:**
1. React imports
2. Third-party libraries (antd, echarts)
3. Local imports (relative paths)

```typescript
import React, { useState, useEffect } from 'react';
import { Row, Col, Card } from 'antd';
import * as echarts from 'echarts';
import { buildApiUrl } from '../../config';
```

**TypeScript Configuration:**
- Target: ES5
- Strict mode: **disabled** (`"strict": false` in tsconfig.json)
- Module: ESNext
- JSX: react-jsx

**Functional Components:**
```typescript
const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats>({...});
  
  useEffect(() => {
    // Effect logic
  }, []);
  
  return (...);
};
```

### Error Handling

**File:** `admin/src/utils/errorHandler.ts`

Error types enum:
```typescript
export enum ErrorType {
  NETWORK = 'NETWORK',
  AUTH = 'AUTH',
  VALIDATION = 'VALIDATION',
  SERVER = 'SERVER',
  UNKNOWN = 'UNKNOWN',
}
```

Static utility class pattern:
```typescript
export class ErrorHandler {
  static handleApiError(error: any, customMessage?: string): void
  static showSuccess(content: string): void
  static confirm(title: string, content?: string): Promise<boolean>
}
```

### API Service Pattern

**File:** `admin/src/services/api.ts`

Singleton pattern with token management:
```typescript
class ApiService {
  private token: string | null = null;
  
  async get<T>(endpoint: string): Promise<ApiResponse<T>>
  async post<T>(endpoint: string, data?: any): Promise<ApiResponse<T>>
  async uploadFile(file: File, uploadToken?: string): Promise<ApiResponse<any>>
}

export const apiService = new ApiService();
```

### Styling

- Inline styles with TypeScript objects (no CSS modules or styled-components)
- Ant Design components with style props

```tsx
<div style={{ 
  display: 'flex', 
  justifyContent: 'center',
  minHeight: '100vh' 
}}>
```

---

## Common Patterns

### Dependency Injection

**Go:** Constructor injection for repositories and handlers
**React:** Context not used; props drilling or service singletons

### Configuration

**Go:** Viper-based YAML config with environment variable override
**React:** Environment variables with `REACT_APP_` prefix

### Security

- Passwords hashed with bcrypt (Go)
- JWT tokens for authentication
- CORS whitelist with wildcard pattern support
- Input validation with `go-playground/validator`

---

## Where to Add New Code

**New API Endpoint:**
1. Add handler method to appropriate handler file in `api/internal/handlers/`
2. Register route in `api/cmd/server/main.go` `setupRoutes()`
3. Add Swagger annotations

**New Database Entity:**
1. Add model to `api/internal/models/models.go` or new file
2. Create repository in `api/internal/database/`
3. Initialize in `main.go`

**New Admin Page:**
1. Create component in `admin/src/components/pages/`
2. Add route in `admin/src/App.tsx`
3. Add menu item if needed

**New API Service Method:**
1. Add method to `admin/src/services/api.ts` ApiService class

---

*Convention analysis: 2026-03-17*
