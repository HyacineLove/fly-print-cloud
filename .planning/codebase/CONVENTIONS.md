# Coding Conventions

**Analysis Date:** 2025-03-18

## Project Structure

This codebase contains two distinct projects:
- **Frontend Admin (`admin/`)**: React + TypeScript SPA
- **Backend API (`api/`)**: Go REST API service

## Frontend Conventions (React/TypeScript)

### Naming Patterns

**Files:**
- Components: PascalCase (e.g., `Dashboard.tsx`, `ErrorBoundary.tsx`)
- Utilities: camelCase (e.g., `errorHandler.ts`, `config.ts`)
- Services: camelCase (e.g., `api.ts`)

**Functions/Variables:**
- Functions: camelCase (e.g., `handleError`, `buildApiUrl`)
- Variables: camelCase (e.g., `apiService`, `dashboardService`)
- React hooks: camelCase with `use` prefix (e.g., `useState`, `useEffect`)

**Types/Interfaces:**
- Interfaces: PascalCase (e.g., `User`, `ApiResponse`, `ErrorType`)
- Enums: PascalCase (e.g., `ErrorType`)
- Type aliases: PascalCase (e.g., `LoadingProps`)

**CSS:**
- Inline styles used exclusively via Ant Design components
- No CSS/SCSS files detected in codebase

### Code Style

**Formatting:**
- No Prettier configuration detected
- Uses Create React App defaults
- 2-space indentation observed in source files

**Linting:**
- ESLint configured via `admin/package.json`:
```json
"eslintConfig": {
  "extends": [
    "react-app",
    "react-app/jest"
  ]
}
```

**TypeScript Configuration (`admin/tsconfig.json`):**
- Target: ES5
- Strict mode: **OFF** (`"strict": false`)
- Module: ESNext
- JSX: react-jsx

### Import Organization

**Order (observed pattern):**
1. React imports
2. Third-party libraries (antd, react-router-dom)
3. Icons (@ant-design/icons)
4. Internal components
5. Utilities/services
6. Type imports

**Example from `admin/src/App.tsx`:**
```typescript
import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import { DashboardOutlined } from '@ant-design/icons';
import Dashboard from './components/pages/Dashboard';
import { handleError } from './utils/errorHandler';
```

### Error Handling

**Frontend Pattern (`admin/src/utils/errorHandler.ts`):**
- Centralized `ErrorHandler` class with static methods
- Error types defined as enum: `NETWORK`, `AUTH`, `VALIDATION`, `SERVER`, `UNKNOWN`
- HTTP status code mapping to user-friendly messages (Chinese)
- Uses Ant Design `message` component for notifications

```typescript
// Usage pattern
import { handleError, showSuccess } from '../utils/errorHandler';

try {
  await apiService.post('/endpoint', data);
  showSuccess('操作成功');
} catch (error) {
  handleError(error);
}
```

**Error Boundary Pattern (`admin/src/components/ErrorBoundary.tsx`):**
- Class-based React Error Boundary
- Displays fallback UI for runtime errors
- Shows error details in development mode only

### Logging

**Frontend:**
- Uses `console.error` for error logging
- No structured logging framework detected
- Ant Design message API for user-facing messages

### Comments

**Language:**
- Mixed Chinese and English comments
- Chinese comments dominate in business logic
- English used for technical explanations

**Style:**
- JSDoc/TSDoc used sparingly
- Section comments with `//` separators
- TODO comments present (e.g., in ErrorBoundary)

### Component Design

**React Components:**
- Functional components with hooks (primary pattern)
- Class components only for Error Boundaries
- Props interface naming: `{ComponentName}Props`

**Example pattern:**
```typescript
interface LoadingProps extends SpinProps {
  fullscreen?: boolean;
  tip?: string;
}

const Loading: React.FC<LoadingProps> = ({ fullscreen = false, tip = '加载中...' }) => {
  // implementation
};
```

### API Service Pattern

**Singleton Pattern (`admin/src/services/api.ts`):**
- Single `ApiService` class instance exported
- JWT token management built-in
- Generic request methods: `get`, `post`, `put`, `delete`
- Custom `ApiError` class for typed errors

## Backend Conventions (Go)

### Naming Patterns

**Files:**
- snake_case for multi-word files (e.g., `user_handler.go`, `token_manager.go`)

**Types/Structs:**
- PascalCase with descriptive names (e.g., `EdgeNode`, `PrintJob`)
- Interface names: descriptive nouns

**Functions:**
- PascalCase for exported functions
- camelCase for unexported (private) functions
- Constructor pattern: `New{Type}` (e.g., `NewTokenManager`)

**Variables:**
- camelCase for local variables
- Descriptive, full words preferred

### Package Organization

**Structure:**
```
api/
  cmd/server/        # Application entry point
  internal/
    auth/           # Authentication services
    config/         # Configuration management
    database/       # Database repositories
    handlers/       # HTTP handlers
    logger/         # Logging utilities
    middleware/     # HTTP middleware
    models/         # Data models
    security/       # Security utilities
    utils/          # General utilities
    websocket/      # WebSocket handlers
```

### Error Handling

**Error Codes (`api/internal/handlers/errors.go`):**
- Numeric error codes organized by domain:
  - 1000-1999: General errors
  - 2000-2099: User errors
  - 3000-3099: Edge Node errors
  - 4000-4099: Printer errors
  - 5000-5099: Print Job errors
  - 6000-6099: File errors
  - 7000-7099: OAuth2 errors

**Response Pattern (`api/internal/handlers/response.go`):**
```go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func SuccessResponse(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, Response{
        Code:    http.StatusOK,
        Message: "success",
        Data:    data,
    })
}
```

### Logging

**Framework:** Uber Zap (`go.uber.org/zap`)

**Pattern (`api/internal/logger/logger.go`):**
- Global `Logger` and `Sugar` instances
- Structured logging with fields
- Helper functions: `Info`, `Error`, `Warn`, `Debug`, `Fatal`

```go
logger.Info("Starting application",
    zap.String("name", cfg.App.Name),
    zap.String("version", cfg.App.Version),
)
```

### Model Definition

**Pattern (`api/internal/models/models.go`):**
- JSON tags for all fields
- Pointer types for nullable fields (`*string`, `*float64`)
- Snake_case JSON keys
- Timestamps: `CreatedAt`, `UpdatedAt`
- Soft delete: `DeletedAt *time.Time`

```go
type Printer struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Status       string   `json:"status"`
    IPAddress    *string  `json:"ip_address"`  // Nullable
    CreatedAt    time.Time `json:"created_at"`
}
```

### Handler Pattern

**Structure:**
- Handler struct with dependencies injected
- Constructor function returning pointer
- Methods receive `*gin.Context`

```go
type UserHandler struct {
    userRepo *database.UserRepository
}

func NewUserHandler(userRepo *database.UserRepository) *UserHandler {
    return &UserHandler{userRepo: userRepo}
}

func (h *UserHandler) ListUsers(c *gin.Context) {
    // handler logic
}
```

### Middleware Pattern

**Structure (`api/internal/middleware/common.go`):**
- Functions return `gin.HandlerFunc`
- Configuration passed as parameters
- Chained via `c.Next()`

### Comments

**Language:**
- Primarily Chinese comments
- English for technical documentation (Swagger comments)
- Package-level comments in Chinese

**Style:**
- Section headers with descriptive comments
- Inline comments explain business logic

## Cross-Cutting Patterns

### Configuration

**Frontend:**
- Environment variables with `REACT_APP_` prefix
- `config.ts` for URL building utilities
- Runtime environment detection

**Backend:**
- Viper for configuration management (`api/internal/config/config.go`)
- YAML configuration files
- Environment variable overrides

### Authentication

**Frontend:**
- JWT token stored in cookies
- OAuth2 integration (Keycloak or builtin)
- Token refresh via `/auth/me` endpoint

**Backend:**
- OAuth2 Resource Server pattern
- Scope-based access control (`admin:*`, `print:submit`, `edge:*`)
- JWT validation middleware

---

*Convention analysis: 2025-03-18*
