# Testing Patterns

**Analysis Date:** 2026-03-17

## Current State

**No tests exist in this codebase.** This document describes the testing setup, recommended patterns, and how to add tests.

---

## Backend Testing (Go)

### Testing Framework

**Standard:** Go's built-in `testing` package
**Recommended additions:**
- `github.com/stretchr/testify` - Assertions and test utilities
- `github.com/DATA-DOG/go-sqlmock` - Database mocking
- `github.com/golang/mock` or `gomock` - Interface mocking

### Test File Locations

**Co-located with source files:**
```
api/internal/handlers/user_handler.go
api/internal/handlers/user_handler_test.go

api/internal/database/user_repository.go
api/internal/database/user_repository_test.go
```

### Test Structure

```go
package handlers

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestUserHandler_CreateUser(t *testing.T) {
    // Arrange
    mockRepo := new(mockUserRepository)
    handler := NewUserHandler(mockRepo)
    
    // Act
    // ... execute handler
    
    // Assert
    assert.Equal(t, expected, actual)
}

func TestUserHandler_CreateUser_ValidationError(t *testing.T) {
    // Test error cases separately
}
```

### Repository Testing Pattern

**File:** Would be in `api/internal/database/*_test.go`

```go
func TestUserRepository_CreateUser(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()
    
    repo := NewUserRepository(&DB{db})
    
    // Set expectations
    mock.ExpectQuery("INSERT INTO users").
        WithArgs("testuser", "test@test.com", sqlmock.AnyArg(), "admin", "active").
        WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
            AddRow("uuid", time.Now(), time.Now()))
    
    user := &models.User{
        Username: "testuser",
        Email:    "test@test.com",
        // ...
    }
    
    err = repo.CreateUser(user)
    assert.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### Handler Testing Pattern

**File:** Would be in `api/internal/handlers/*_test.go`

```go
func TestUserHandler_ListUsers(t *testing.T) {
    // Setup Gin test context
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    
    // Mock repository
    mockRepo := new(MockUserRepository)
    mockRepo.On("ListUsers", 0, 10).Return([]*models.User{{ID: "1", Username: "test"}}, 1, nil)
    
    handler := NewUserHandler(mockRepo)
    handler.ListUsers(c)
    
    assert.Equal(t, 200, w.Code)
    // Assert response body
}
```

### Integration Testing

**Location:** `api/tests/integration/`

```go
func TestAPI_CreateUserIntegration(t *testing.T) {
    // Start test server with test database
    // Make HTTP requests
    // Assert responses
}
```

### Recommended Test Commands

```bash
# Run all tests
cd api && go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/handlers/...

# Verbose output
go test -v ./...

# Race detection
go test -race ./...
```

---

## Frontend Testing (React/TypeScript)

### Testing Framework

**Currently configured:**
- Jest (via react-scripts)
- React Testing Library (`@testing-library/react`)
- User Event (`@testing-library/user-event`)
- Jest DOM matchers (`@testing-library/jest-dom`)

**Configuration:** Already in `admin/package.json`:
```json
"scripts": {
  "test": "react-scripts test"
},
"eslintConfig": {
  "extends": [
    "react-app",
    "react-app/jest"
  ]
}
```

### Test File Locations

**Co-located with components:**
```
admin/src/components/pages/Dashboard.tsx
admin/src/components/pages/Dashboard.test.tsx

admin/src/services/api.ts
admin/src/services/api.test.ts
```

### Component Testing Pattern

```typescript
// Dashboard.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Dashboard from './Dashboard';

// Mock the fetch API
global.fetch = jest.fn();

describe('Dashboard', () => {
  beforeEach(() => {
    (fetch as jest.Mock).mockClear();
  });

  it('renders loading state initially', () => {
    render(
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>
    );
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('displays stats after loading', async () => {
    (fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        code: 200,
        data: { items: [] },
        pagination: { total: 0 }
      })
    });

    render(
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('打印机总数')).toBeInTheDocument();
    });
  });
});
```

### Service Testing Pattern

```typescript
// api.test.ts
import { apiService } from './api';

describe('ApiService', () => {
  beforeEach(() => {
    fetch.mockClear();
  });

  describe('get', () => {
    it('makes GET request with auth header', async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ code: 200, data: {} })
      });

      await apiService.get('/admin/users');

      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('/admin/users'),
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            'Content-Type': 'application/json'
          })
        })
      );
    });
  });
});
```

### Error Handler Testing

```typescript
// errorHandler.test.ts
import { ErrorHandler, ErrorType } from './errorHandler';

describe('ErrorHandler', () => {
  describe('getErrorType', () => {
    it('returns AUTH for 401', () => {
      expect(ErrorHandler.getErrorType(401)).toBe(ErrorType.AUTH);
    });

    it('returns SERVER for 500', () => {
      expect(ErrorHandler.getErrorType(500)).toBe(ErrorType.SERVER);
    });
  });
});
```

### Recommended Test Commands

```bash
# Run all tests
cd admin && npm test

# Run in CI mode
npm test -- --watchAll=false

# Run with coverage
npm test -- --coverage

# Run specific file
npm test -- Dashboard.test.tsx
```

---

## Test Coverage Goals

### Backend (Go)

**Critical paths to cover:**
- `api/internal/handlers/*` - HTTP handlers (highest priority)
- `api/internal/database/*` - Repository layer
- `api/internal/auth/*` - Authentication logic
- `api/internal/security/*` - Token validation, encryption
- `api/internal/middleware/*` - Auth middleware, CORS

**Recommended coverage targets:**
- Handlers: 80%+
- Repositories: 70%+
- Services (auth, security): 90%+
- Utilities: 60%+

### Frontend (React)

**Critical paths to cover:**
- `admin/src/services/api.ts` - API communication
- `admin/src/utils/errorHandler.ts` - Error handling logic
- `admin/src/components/ErrorBoundary.tsx` - Error boundary
- Form validation logic in page components

**Recommended coverage targets:**
- Services: 80%+
- Utilities: 70%+
- Components: 50%+ (critical paths only)

---

## Mocking Strategy

### Backend

**Database:**
- Use `sqlmock` for repository tests
- Use in-memory SQLite for integration tests

**External Services:**
- Mock OAuth2 provider responses
- Mock WebSocket connections
- Mock file storage operations

### Frontend

**API Calls:**
```typescript
// Mock global fetch
global.fetch = jest.fn();

// Setup mock response
(fetch as jest.Mock).mockResolvedValueOnce({
  ok: true,
  json: async () => ({ code: 200, data: mockData })
});
```

**Ant Design Components:**
```typescript
jest.mock('antd', () => ({
  message: {
    error: jest.fn(),
    success: jest.fn()
  },
  Modal: {
    confirm: jest.fn()
  }
}));
```

---

## E2E Testing

**Not currently configured.** Recommended setup:

### Options

1. **Cypress** - Full E2E testing
2. **Playwright** - Modern E2E testing
3. **Go test with httptest** - API-level integration

### Recommended E2E Scenarios

- User login flow
- Create print job end-to-end
- Edge node registration flow
- File upload and download

---

## Adding New Tests

### Backend

**1. Create test file:**
```bash
touch api/internal/handlers/user_handler_test.go
```

**2. Add dependencies:**
```bash
cd api && go get github.com/stretchr/testify
```

**3. Write tests following patterns above**

### Frontend

**1. Create test file alongside component:**
```bash
touch admin/src/components/pages/Users.test.tsx
```

**2. Run tests:**
```bash
cd admin && npm test
```

---

## CI/CD Integration

**Recommended GitHub Actions workflow:**

```yaml
name: Tests
on: [push, pull_request]

jobs:
  backend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - run: cd api && go test -v -race -cover ./...

  frontend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: cd admin && npm ci
      - run: npm test -- --watchAll=false --coverage
```

---

## Testing Gaps

**Current untested areas (high priority):**

1. **Authentication flow** - `api/internal/auth/`, `api/internal/middleware/oauth2.go`
2. **Token management** - `api/internal/security/token_manager.go`
3. **Database operations** - All repository methods
4. **WebSocket handling** - `api/internal/websocket/`
5. **File upload/download** - `api/internal/handlers/file_handler.go`
6. **API error handling** - Frontend error handler utility

---

*Testing analysis: 2026-03-17*
