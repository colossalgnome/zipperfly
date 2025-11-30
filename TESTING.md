# Testing Documentation

## Test Suite Overview

Zipperfly includes a comprehensive test suite covering critical components with unit and integration tests.

## Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with verbose output
make test-verbose

# Generate HTML coverage report
make test-coverage-html
```

Or using `go test` directly:

```bash
# All tests
go test ./...

# Specific package
go test ./internal/config

# With coverage
go test ./... -cover

# Verbose with race detection
go test ./... -v -race
```

## Test Coverage

### ✅ Fully Tested (100% coverage)

**`internal/auth`** - Signature verification
- Valid/invalid signatures
- Expiry handling
- HMAC generation
- Enforce signing mode
- Metrics tracking

**`internal/models`** - Data models
- ByteCounter write operations
- Byte counting accuracy
- Concurrent writes

### ✅ Well Tested (80%+ coverage)

**`internal/circuitbreaker`** - Circuit breaker (83.3%)
- Circuit state transitions (closed → open → half-open)
- Failure threshold handling
- Recovery after timeout
- Metrics integration

### ✅ Partially Tested (40%+ coverage)

**`internal/storage`** - Storage providers (42.3%)
- Local filesystem operations
- Path traversal security
- Health checks
- File fetching
- *Missing: S3 provider tests, retry logic*

**`internal/config`** - Configuration (37.3%)
- Duration parsing
- Byte size parsing (K/M/G/T suffixes)
- Integer parsing
- String list parsing
- *Missing: Full config loading tests*

### ⚠️ Light Testing (< 40% coverage)

**`internal/handlers`** - HTTP handlers (19.4%)
- Health endpoint (all states)
- Request ID middleware
- *Missing: Download handler, callback retry*

### ❌ Not Yet Tested (0% coverage)

- `internal/database` - Database stores (postgres, mysql, redis)
- `internal/server` - HTTP server setup
- `internal/metrics` - Metrics definitions (initialization only, usage tested elsewhere)
- `cmd/server` - Main entry point

## Test Structure

### Unit Tests

Tests for individual functions and methods in isolation:

- **Config parsers** (`config_test.go`) - Parse duration, bytes, integers, string lists
- **Signature verification** (`signature_test.go`) - HMAC, expiry, enforcement
- **ByteCounter** (`models_test.go`) - Byte counting logic
- **Circuit breaker** (`breaker_test.go`) - State machine logic

### Integration Tests

Tests that combine multiple components:

- **Health handler** (`health_test.go`) - Database + storage + HTTP response
- **Local storage** (`local_test.go`) - File I/O + circuit breaker + metrics
- **Request ID** (`requestid_test.go`) - Middleware + context propagation

## Test Patterns

### Table-Driven Tests

Most tests use table-driven patterns for comprehensive coverage:

```go
tests := []struct {
    name    string
    input   string
    want    expected
    wantErr bool
}{
    {name: "case1", input: "value", want: result},
    {name: "case2", input: "other", wantErr: true},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```

### Mocking

Mock implementations for external dependencies:

- **mockDB** - Database store mock (in `health_test.go`)
- **mockStorage** - Storage provider mock (in `health_test.go`)

### Shared Metrics

To avoid Prometheus duplicate registration errors, tests use shared metrics instances:

```go
var sharedMetrics = metrics.New()
```

## What Needs More Testing

### High Priority

1. **Database Stores** - Critical for production
   - Connection handling
   - Query execution
   - Timeout behavior
   - Error handling
   - All three backends: Postgres, MySQL, Redis

2. **Download Handler** - Core functionality
   - ZIP generation
   - Multi-file handling
   - Error handling
   - Missing file handling (IGNORE_MISSING)
   - Resource limits (MAX_FILES_PER_REQUEST, MAX_FILE_SIZE)
   - Context cancellation
   - Metrics tracking

3. **Callback Logic** - Important for workflows
   - Retry with exponential backoff
   - Success/failure tracking
   - Metrics

### Medium Priority

4. **S3 Storage Provider**
   - Connection handling
   - Object fetching
   - Retry logic
   - Health checks
   - Circuit breaker integration

5. **Server Setup**
   - Router configuration
   - Middleware chain
   - HTTPS/Let's Encrypt
   - Graceful shutdown

### Low Priority

6. **Main Entry Point** - Hard to test, usually covered by integration tests
7. **Metrics Definitions** - Auto-registered, hard to test in isolation

## Adding New Tests

When adding tests:

1. Use table-driven tests for multiple cases
2. Test both success and error paths
3. Use `t.TempDir()` for filesystem tests
4. Share metrics instances to avoid registration conflicts
5. Use descriptive test names
6. Add edge cases and boundary conditions

Example:

```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   interface{}
        want    interface{}
        wantErr bool
    }{
        {name: "success case", input: x, want: y},
        {name: "error case", input: z, wantErr: true},
        {name: "edge case", input: edge, want: result},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewFeature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Continuous Integration

Tests should be run on every commit. Example GitHub Actions workflow:

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test
      - run: make test-coverage
```

## Performance Testing

For load testing and benchmarking:

```bash
# Benchmarks (when added)
go test -bench=. ./...

# Memory profiling
go test -memprofile=mem.prof ./...

# CPU profiling
go test -cpuprofile=cpu.prof ./...
```
