# Testing Patterns

**Analysis Date:** 2025-03-18

## Overview

**Critical Finding: No tests detected in this codebase.**

This codebase currently has **zero test coverage**. No unit tests, integration tests, or end-to-end tests were found in either the frontend (`admin/`) or backend (`api/`) components.

## Frontend Testing Status

### Test Framework

**Installed but Unused:**
- **Testing Library**: `@testing-library/jest-dom@^5.16.4`, `@testing-library/react@^13.3.0`, `@testing-library/user-event@^13.5.0`
- **Jest**: Included via `react-scripts` (Create React App)
- **Types**: `@types/jest@^27.5.2`

**Configuration:**
- ESLint extends `react-app/jest` (configured in `admin/package.json`)
- No custom Jest configuration detected
- No test scripts beyond default CRA `react-scripts test`

### Test File Locations

**Expected but Not Found:**
- No `*.test.ts` or `*.test.tsx` files
- No `*.spec.ts` or `*.spec.tsx` files
- No `__tests__/` directories
- No `src/setupTests.ts` or similar setup files

### Run Commands (Standard CRA)

```bash
# From admin/ directory
npm test              # Run tests in watch mode
npm test -- --coverage # Run with coverage report
npm test -- --watchAll=false # Run once (CI mode)
```

**Note:** Commands exist but will report "No tests found" if executed.

## Backend Testing Status

### Test Framework

**Not Configured:**
- No `*_test.go` files found
- No testing dependencies in `go.mod`
- No test utilities or helper packages

**Standard Go Testing Available:**
- Go's built-in `testing` package is available
- No external testing libraries (testify, ginkgo, etc.)

### Test File Locations

**Expected but Not Found:**
- No `*_test.go` files in any package
- No testdata directories
- No benchmark files (`*_benchmark.go`)

### Run Commands (Standard Go)

```bash
# From api/ directory
go test ./...         # Run all tests
go test -v ./...      # Run with verbose output
go test -cover ./...  # Run with coverage
go test -race ./...   # Run with race detector
```

**Note:** Commands will report "no test files" if executed.

## Testing Recommendations

### Priority Areas for Testing

**1. Backend API Handlers (`api/internal/handlers/`)**
Files to test:
- `user_handler.go` - User CRUD operations
- `edge_node_handler.go` - Edge node management
- `printer_handler.go` - Printer operations
- `print_job_handler.go` - Print job lifecycle
- `oauth2_handler.go` - Authentication flows
- `file_handler.go` - File upload/download

**2. Critical Business Logic**
Files to test:
- `api/internal/security/token_manager.go` - Token generation/validation
- `api/internal/auth/builtin_auth_service.go` - Authentication service
- `api/internal/websocket/manager.go` - WebSocket connection management

**3. Frontend Components (`admin/src/components/`)**
Components to test:
- `ErrorBoundary.tsx` - Error handling behavior
- `Loading.tsx` - Loading state rendering
- `pages/Login.tsx` - Authentication flow
- `pages/Dashboard.tsx` - Data fetching and display

**4. Service Layer (`admin/src/services/`)**
- `api.ts` - API client methods, error handling

### Recommended Testing Approach

#### Backend (Go)

**Unit Tests for Handlers:**
```go
// api/internal/handlers/user_handler_test.go
package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestUserHandler_ListUsers(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    // Setup mock repository
    mockRepo := new(MockUserRepository)
    mockRepo.On("ListUsers").Return([]models.User{}, nil)
    
    handler := NewUserHandler(mockRepo)
    
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    
    handler.ListUsers(c)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

**Integration Tests:**
- Test database repository methods with real database
- Test middleware chains
- Test WebSocket connections

#### Frontend (React/TypeScript)

**Component Tests:**
```typescript
// admin/src/components/Loading.test.tsx
import { render, screen } from '@testing-library/react';
import Loading from './Loading';

describe('Loading', () => {
  it('renders fullscreen loading', () => {
    render(<Loading fullscreen tip="Loading..." />);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders inline loading', () => {
    render(<Loading tip="Saving..." />);
    expect(screen.getByText('Saving...')).toBeInTheDocument();
  });
});
```

**Service Tests:**
```typescript
// admin/src/services/api.test.ts
import apiService from './api';

describe('ApiService', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
  });

  it('makes GET request', async () => {
    fetchMock.mockResponseOnce(JSON.stringify({ code: 200, data: [] }));
    
    const result = await apiService.get('/test');
    
    expect(result.code).toBe(200);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/test'),
      expect.any(Object)
    );
  });
});
```

## Test Structure Guidelines

### When Adding Tests, Follow These Patterns:

**Backend Test Organization:**
```
api/
  internal/
    handlers/
      user_handler.go
      user_handler_test.go      # Co-located test file
    database/
      user_repository.go
      user_repository_test.go
```

**Frontend Test Organization:**
```
admin/src/
  components/
    Loading.tsx
    Loading.test.tsx           # Co-located test file
    pages/
      Dashboard.tsx
      Dashboard.test.tsx
```

### Naming Conventions for Tests

**Go:**
- Test functions: `Test{FunctionName}` (e.g., `TestUserHandler_ListUsers`)
- Table-driven tests for multiple cases
- Subtests with `t.Run()` for organization

**TypeScript:**
- Test files: `{filename}.test.ts` or `{filename}.spec.ts`
- Describe blocks: Component or function name
- Test cases: descriptive strings

## Mocking Strategy

### Backend (Go)

**Repository Mocking:**
```go
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) GetUserByID(id string) (*models.User, error) {
    args := m.Called(id)
    return args.Get(0).(*models.User), args.Error(1)
}
```

**HTTP Testing:**
- Use `gin.CreateTestContext()` for handler tests
- Use `httptest.NewRecorder()` for response capture

### Frontend (TypeScript)

**Fetch Mocking:**
```typescript
// Use jest-fetch-mock or similar
global.fetch = jest.fn();
```

**Service Mocking:**
```typescript
jest.mock('../services/api', () => ({
  get: jest.fn().mockResolvedValue({ code: 200, data: [] }),
}));
```

## Coverage Goals

**Recommended Minimum Coverage:**
- Handlers: 80%
- Services: 70%
- Utilities: 60%
- Components: 50%

**Critical Paths to Cover:**
1. Authentication flows (login, logout, token refresh)
2. Print job lifecycle (create, dispatch, complete, fail)
3. File upload/download
4. Edge node registration and heartbeat
5. Error handling paths

## CI/CD Integration

**When Tests Are Added:**

Add to GitHub Actions or similar:
```yaml
- name: Run Backend Tests
  working-directory: ./api
  run: go test -v -race -coverprofile=coverage.out ./...

- name: Run Frontend Tests
  working-directory: ./admin
  run: npm test -- --coverage --watchAll=false
```

## Testing Tools to Consider

**Backend:**
- `github.com/stretchr/testify` - Assertions and mocks
- `github.com/DATA-DOG/go-sqlmock` - Database mocking
- `github.com/gavv/httpexpect` - HTTP testing

**Frontend:**
- `msw` (Mock Service Worker) - API mocking
- `@testing-library/react-hooks` - Hook testing
- `jest-fetch-mock` - Fetch mocking

## Current Testing Gaps

**High Risk - No Tests:**
- `admin/src/services/api.ts` - Core API communication
- `api/internal/security/token_manager.go` - Security-critical code
- `api/internal/websocket/manager.go` - Real-time communication
- `api/internal/handlers/oauth2_handler.go` - Authentication

**Medium Risk - No Tests:**
- All React components
- Database repository methods
- Configuration loading
- Middleware chains

**Low Risk - No Tests:**
- Utility functions
- Static configuration
- Type definitions

---

*Testing analysis: 2025-03-18*
