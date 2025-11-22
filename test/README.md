# Testing Guide

## Test Structure

The project uses PostgreSQL transactions for database tests to ensure complete isolation and automatic cleanup.

```
test/
├── testdb/           # Database test helpers
├── integration_test.go
└── README.md

internal/*/
└── *_test.go         # Unit tests per package
```

## Running Tests

### Quick Unit Tests (No Database)

```bash
make test
```

### Full Tests with PostgreSQL

```bash
make test-db
```

This will:
1. Start PostgreSQL test container (port 5433)
2. Wait for database to be ready
3. Run all tests in transactions (automatic rollback)
4. Clean up containers

### Coverage Report

```bash
make test-coverage
```

Generates `coverage.html` with detailed coverage report.

### Manual Test with Local PostgreSQL

```bash
# Setup test database
createdb trader_test
psql trader_test < migrations/001_init.sql

# Run tests
export TEST_DATABASE_URL="host=localhost port=5432 user=trader password=trader dbname=trader_test sslmode=disable"
go test -v ./...
```

## Writing Database Tests

### Using Transaction Rollback

```go
package mypackage

import (
    "testing"
    "github.com/alexanderselivanov/trader/test/testdb"
)

func TestMyFunction(t *testing.T) {
    // Setup test database with automatic rollback
    db := testdb.Setup(t)
    
    // All database operations happen in transaction
    db.Exec(t, "INSERT INTO users ...")
    
    // Test your code
    result := MyFunction(db.GetTx())
    
    // Assertions
    if result != expected {
        t.Error("...")
    }
    
    // Cleanup is automatic - transaction rolls back at test end
}
```

### Seed Test Data

```go
func TestWithSeedData(t *testing.T) {
    db := testdb.Setup(t)
    
    // Seed common test data
    db.SeedTestData(t)
    
    // Now you have test user with ID=1, config, and state
    
    // Run your tests
}
```

### Create Custom Test Data

```go
func TestCustomData(t *testing.T) {
    db := testdb.Setup(t)
    
    // Create test user
    userID := db.CreateTestUser(t, 123456, "testuser")
    
    // Create test config
    db.CreateTestConfig(t, userID, "BTC/USDT", 1000.0)
    
    // Run tests
}
```

## Test Categories

### Unit Tests
- No external dependencies
- Fast execution
- Mock all dependencies

Examples:
- `internal/risk/circuit_breaker_test.go`
- `internal/risk/position_sizer_test.go`
- `internal/sentiment/analyzer_test.go`

### Integration Tests
- Use test database
- Test component interactions
- Automatic rollback

Examples:
- `internal/users/repository_test.go`
- `test/integration_test.go`

### End-to-End Tests
- Full system test
- Mock exchange only
- Real database (test instance)

## Environment Variables

```bash
# Test database connection
TEST_DATABASE_URL="host=localhost port=5433 user=trader password=trader dbname=trader_test sslmode=disable"

# Skip integration tests
go test -short ./...
```

## Assertions

Use testdb helpers for common assertions:

```go
db.AssertUserExists(t, telegramID)
db.AssertConfigExists(t, userID, symbol)
db.AssertTradeCount(t, userID, expectedCount)
```

## Best Practices

1. **Use transactions**: All database tests run in transactions with automatic rollback
2. **Isolation**: Each test is completely isolated
3. **Cleanup**: No manual cleanup needed
4. **Parallel**: Tests can run in parallel with `-p` flag
5. **Fast**: Transaction rollback is faster than truncating tables

## Continuous Integration

For CI/CD, use docker-compose:

```yaml
# .github/workflows/test.yml
- name: Start test database
  run: docker-compose -f docker-compose.test.yml up -d

- name: Run tests
  run: ./scripts/test_with_db.sh
```

## Troubleshooting

**Tests fail to connect to database:**
- Check PostgreSQL is running
- Verify TEST_DATABASE_URL is correct
- Check port is not in use (5433)

**Tests pass locally but fail in CI:**
- Ensure migrations are applied
- Check database version matches
- Verify environment variables

**Slow tests:**
- Use `-short` flag to skip integration tests
- Run specific package: `go test ./internal/users/...`
- Use `-p 1` to run serially if parallel tests conflict

## Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

Minimum coverage targets:
- Critical packages (risk, portfolio): 80%+
- Adapters: 60%+
- Overall project: 70%+

