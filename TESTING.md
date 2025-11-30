# Testing Documentation

## Test Suite Overview

Zipperfly includes a comprehensive test suite covering all critical components with both unit and integration tests. The integration tests validate all combinations of database backends (PostgreSQL, MySQL, Redis) with storage providers (local filesystem, S3/MinIO).

## Running Tests

### Unit Tests

```bash
# Run all unit tests (fast, no external dependencies)
make test

# Run with coverage
make test-coverage

# Generate HTML coverage report
make test-coverage-html
```

### Integration Tests

Integration tests require Docker services (PostgreSQL, MySQL, Redis, MinIO).

```bash
# Setup test environment (Docker Compose)
make test-integration-setup

# Run integration tests (requires Docker services)
make test-integration

# Run full pipeline (setup + tests)
make test-integration-full
```

Or using `go test` directly:

```bash
# Unit tests only
go test -short ./...

# Integration tests (requires -tags=integration)
go test -tags=integration ./test/integration -v
```

## Current Test Coverage

**Total Coverage: 66.6%**

### ✅ Fully Tested (100% coverage)

**`internal/auth`** - Signature verification
- Valid/invalid signatures
- Expiry handling (past, future)
- HMAC-SHA256 generation
- Enforce signing mode
- Metrics tracking (signature failures, expired requests)

**`internal/models`** - Data models
- ByteCounter write operations
- Byte counting accuracy
- Concurrent writes with race detection

**`internal/metrics`** - Metrics definitions and initialization
- Singleton pattern
- Prometheus auto-registration
- Runtime metrics collector (memory, goroutines)
- All metric fields (counters, histograms, gauges)

### ✅ Well Tested (80%+ coverage)

**`internal/handlers`** (87.3%) - HTTP request handlers
- Download handler (96.8%):
  - Missing ID validation
  - Invalid/expired signature handling
  - Record not found errors
  - Single and multi-file downloads
  - Missing file handling (with/without `IGNORE_MISSING`)
  - ZIP file generation and validation
- Filename preparation (100%):
  - Default names, sanitization
  - `.zip` suffix stripping
  - YMD date appending
- Callback system (100%):
  - Success/failure tracking
  - Exponential backoff retry logic
  - Empty URL handling
- Health endpoint (100%): Database + storage health checks
- Request ID middleware (100%): UUID generation and propagation
- *Missing: BasicAuth middleware (0%)*

**`internal/circuitbreaker`** (83.3%) - Circuit breaker pattern
- Circuit state transitions (closed → open → half-open)
- Failure threshold handling
- Recovery after timeout
- Metrics integration

### ✅ Partially Tested (50%+ coverage)

**`internal/config`** (94.3%) - Configuration loading and parsing
- Environment variable loading
- Duration parsing (s, m, h)
- Byte size parsing (K, M, G, T suffixes)
- Integer parsing with defaults
- String list parsing (comma-separated)
- Full config validation (DB URL, HTTPS domains)
- HTTPS + local storage configurations
- *Missing: A few edge cases (~6% uncovered)*

**`internal/server`** (63.4%) - HTTP server setup
- Router configuration
- Metrics endpoint (with/without auth)
- HTTP server start and graceful shutdown
- Signal handling (SIGINT, SIGTERM)
- *Missing: HTTPS/Let's Encrypt setup (0%)*

**`internal/storage`** (50.0%) - Storage providers
- Local filesystem:
  - File fetching (89.2%)
  - Path traversal security
  - Health checks (75%)
  - Retry logic (44.4%)
- S3/MinIO:
  - Provider initialization (73.7%)
  - Path-style vs virtual-hosted-style addressing
  - *Missing: GetObject (0%), HealthCheck (0%), retry logic (0%)*
- *Missing: Factory method New() (0%)*

### ⚠️ Light Testing (< 50% coverage)

**`internal/database`** (20.5%) - Database stores
- PostgreSQL store initialization
- MySQL store initialization + URL-to-DSN conversion (90%)
- Redis store initialization
- *Missing: GetRecord implementations (covered only in integration tests)*

**`cmd/server`** (21.4%) - Main entry point
- Environment file loading (.env, CONFIG_FILE)
- Explicit config file flag
- Default .env discovery
- *Missing: Main function, server initialization flow*

## Test Structure

### Unit Tests

Tests for individual functions and methods in isolation:

- **Config parsers** (`config_test.go`) - Parse duration, bytes, integers, string lists
- **Signature verification** (`signature_test.go`) - HMAC, expiry, enforcement
- **ByteCounter** (`models_test.go`) - Byte counting logic
- **Circuit breaker** (`breaker_test.go`) - State machine logic
- **Download handler** (`download_test.go`) - ZIP generation, callbacks, file handling
- **Storage providers** (`local_test.go`, `s3_test.go`) - File operations, path security
- **Server** (`server_test.go`) - Metrics endpoint, graceful shutdown
- **Metrics** (`metrics_test.go`) - Singleton, runtime collector

### Integration Tests

Tests that validate full end-to-end workflows with real external services.

**Test Matrix: 6 combinations**

| Database   | Storage | Test Function                       |
|------------|---------|-------------------------------------|
| PostgreSQL | Local   | `TestIntegration_LocalStorage_PostgreSQL` |
| MySQL      | Local   | `TestIntegration_LocalStorage_MySQL`      |
| Redis      | Local   | `TestIntegration_LocalStorage_Redis`      |
| PostgreSQL | S3      | `TestIntegration_S3_PostgreSQL`           |
| MySQL      | S3      | `TestIntegration_S3_MySQL`                |
| Redis      | S3      | `TestIntegration_S3_Redis`                |

**Test Scenarios:**
- Basic single file download
- Multi-file download (3 files)
- All files download (4 files including binary)
- Download with missing file (tests `IGNORE_MISSING` flag)
- Nonexistent download ID (404 error)
- ZIP validation (structure, file count, content)

**Infrastructure:**
- Docker Compose with PostgreSQL, MySQL, Redis, MinIO
- Pre-seeded SQL schemas with test records
- Redis records seeded programmatically
- S3 fixtures uploaded automatically on first run
- Shared metrics instance to avoid Prometheus conflicts

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

- **mockDownloadDB** - Database store mock (in `download_test.go`)
- **mockDownloadStorage** - Storage provider mock (in `download_test.go`)
- **mockDB** - Simple database mock (in `health_test.go`)
- **mockStorage** - Simple storage mock (in `health_test.go`)

### Shared Metrics

To avoid Prometheus duplicate registration errors, tests use shared metrics instances:

```go
var sharedMetrics = metrics.New()
```

The `metrics.New()` function implements a singleton pattern, so multiple calls return the same instance.

## What Needs More Testing

### High Priority

1. **S3 Storage Provider** (currently 0% for core methods)
   - GetObject implementation
   - HealthCheck implementation
   - Retry error handling (isRetryableError)
   - Note: Initialization is tested (73.7%), and integration tests validate end-to-end S3 workflows

2. **Database GetRecord Methods** (currently 0%)
   - PostgreSQL GetRecord (tested in integration only)
   - MySQL GetRecord (tested in integration only)
   - Redis GetRecord (tested in integration only)
   - Note: Unit tests exist but skip in short mode; integration tests provide full coverage

3. **HTTPS/TLS Server** (currently 0%)
   - Let's Encrypt certificate acquisition
   - HTTPS server startup
   - Certificate renewal
   - Note: This is lower priority as most deployments use reverse proxies

### Medium Priority

4. **BasicAuth Middleware** (currently 0%)
   - Username/password validation
   - Unauthorized responses
   - Integration with metrics endpoint

5. **Storage Factory Method** (currently 0%)
   - `storage.New()` dispatcher logic
   - Local vs S3 selection based on config

6. **Main Entry Point** (currently 21.4%)
   - Main function flow
   - Database/storage initialization
   - Server startup
   - Error handling
   - Note: Hard to test; usually covered by manual testing or integration tests

### Low Priority

7. **Config Edge Cases** (5.7% uncovered)
   - Uncommon parsing scenarios
   - Note: Main paths are well-covered (94.3%)

8. **Local Storage Retry Logic** (44.4%)
   - `isLocalRetryableError` edge cases
   - Note: Core logic is tested

## Docker Test Environment

### Services

The integration test environment includes:

- **PostgreSQL 16**: Port 5432, database `zipperfly_test`
- **MySQL 8.3**: Port 3306, database `zipperfly_test`
- **Redis 7**: Port 6379, database 0
- **MinIO**: Ports 9000 (API), 9001 (Console)

### Fixtures

Test data is stored in `test/fixtures/`:

**Database schemas:**
- `postgres_schema.sql` - Table creation + 8 test records
- `mysql_schema.sql` - Table creation + 8 test records

**Test files (in `test/fixtures/files/test-bucket/`):**
- `document.txt` - Multi-line text file
- `data.json` - JSON with array and metadata
- `data.csv` - CSV with 5 sample records
- `binary.dat` - 10KB random binary data

**Test records (8 scenarios):**
1. `test-basic` - Single file
2. `test-multi` - 3 files (txt, json, csv)
3. `test-all` - All 4 files
4. `test-missing` - File + nonexistent file (tests IGNORE_MISSING)
5. `test-password` - Password-protected download
6. `test-callback` - With callback URL
7. `test-headers` - Custom headers
8. `test-large` - Large file scenario

### Managing Test Environment

```bash
# Start services
docker-compose -f docker-compose.test.yml up -d

# Check service health
docker-compose -f docker-compose.test.yml ps

# View logs
docker-compose -f docker-compose.test.yml logs -f

# Stop and remove
docker-compose -f docker-compose.test.yml down

# Remove volumes (clean state)
docker-compose -f docker-compose.test.yml down -v
```

## Adding New Tests

When adding tests:

1. Use table-driven tests for multiple cases
2. Test both success and error paths
3. Use `t.TempDir()` for filesystem tests
4. Share metrics instances to avoid registration conflicts
5. Use descriptive test names
6. Add edge cases and boundary conditions
7. For integration tests, use the `-short` skip pattern:
   ```go
   if testing.Short() {
       t.Skip("skipping integration test")
   }
   ```

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
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test
      - run: make test-coverage

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test-integration-setup
      - run: make test-integration
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

# Race detection (slower but catches concurrency issues)
go test -race ./...
```

## Coverage Trends

Track coverage over time:

```bash
# Generate coverage report
go test -short ./... -coverprofile=coverage.out

# View coverage percentages by package
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Upload to Codecov/Coveralls (in CI)
bash <(curl -s https://codecov.io/bash)
```

## Test Data Management

### Seeding Test Data

**SQL Databases:**
- Test data is loaded automatically by `test/setup-test-env.sh`
- Schemas include CREATE TABLE + INSERT statements
- Run `make test-integration-setup` to reload

**Redis:**
- Data seeded programmatically in test setup functions
- Uses JSON serialization matching production format
- Cleaned up automatically after tests

**S3/MinIO:**
- Files uploaded on first integration test run (lazy initialization)
- Uses `sync.Once` to avoid redundant uploads
- Bucket created automatically if missing

### Updating Test Fixtures

To add new test files:

1. Add file to `test/fixtures/files/test-bucket/`
2. Add corresponding DB record to `postgres_schema.sql` and `mysql_schema.sql`
3. Add record to Redis seed function in `integration_test.go`
4. Files are automatically uploaded to MinIO on next test run

## Debugging Failed Tests

### Common Issues

**Prometheus duplicate registration:**
```
panic: duplicate metrics collector registration attempted
```
- **Fix:** Use shared metrics instance (`var sharedMetrics = metrics.New()`)

**Database connection errors:**
```
MySQL not available: dial tcp 127.0.0.1:3306: connect: connection refused
```
- **Fix:** Start Docker services with `make test-integration-setup`

**S3 upload failures:**
```
failed to seed S3 fixtures: RequestError: send request failed
```
- **Fix:** Ensure MinIO is running and accessible on port 9000

**Test timeout:**
```
test timed out after 10m0s
```
- **Fix:** Check for deadlocks, infinite loops, or slow external calls

### Debugging Tips

```bash
# Run single test with verbose output
go test -v -run TestName ./package

# Skip integration tests
go test -short ./...

# Enable race detection
go test -race ./...

# Increase test timeout
go test -timeout 30m ./...

# Run with debug logging (if implemented)
DEBUG=1 go test -v ./...
```
