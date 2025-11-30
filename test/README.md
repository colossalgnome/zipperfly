# Integration Tests

This directory contains integration tests for zipperfly that test the full stack with real services.

## Overview

Integration tests verify end-to-end functionality using:
- **Real databases**: PostgreSQL, MySQL, Redis (via Docker)
- **Real storage**: MinIO (S3-compatible) and local filesystem
- **Actual file downloads**: Generate real ZIP files with test data

## Quick Start

```bash
# Setup test environment (starts Docker containers)
make test-integration-setup

# Run integration tests
make test-integration

# Or do both
make test-integration-full

# Stop test environment when done
make test-integration-down
```

## Test Environment

### Docker Services

The test environment uses `docker-compose.test.yml` to spin up:

| Service    | Port | Credentials                  | Purpose                      |
|------------|------|------------------------------|------------------------------|
| PostgreSQL | 5432 | zipperfly/testpass           | Primary database testing     |
| MySQL      | 3306 | zipperfly/testpass           | MySQL compatibility testing  |
| Redis      | 6379 | (no auth)                    | Redis backend testing        |
| MinIO      | 9000 | minioadmin/minioadmin        | S3-compatible storage testing|

### Test Data

**`fixtures/files/`** - Sample files for download testing:
- `document.txt` - Plain text (UTF-8)
- `data.json` - JSON data
- `data.csv` - CSV data
- `binary.dat` - Binary data (10KB)

**`fixtures/postgres_schema.sql`** - PostgreSQL test schema and data
**`fixtures/mysql_schema.sql`** - MySQL test schema and data

### Test Records

Pre-populated download records for testing:

| ID                | Files                                        | Features                      |
|-------------------|----------------------------------------------|-------------------------------|
| `test-basic`      | document.txt                                 | Single file                   |
| `test-multi`      | document.txt, data.json, data.csv            | Multiple files                |
| `test-binary`     | binary.dat                                   | Binary data                   |
| `test-all`        | All 4 files                                  | Large download                |
| `test-missing`    | document.txt, nonexistent.txt                | Partial failure (IGNORE_MISSING) |
| `test-password`   | document.txt                                 | Password-protected ZIP        |
| `test-callback`   | document.txt                                 | Callback URL                  |
| `test-headers`    | document.txt                                 | Custom HTTP headers           |

## Manual Testing

### Setup Environment

```bash
# Start all services
docker-compose -f docker-compose.test.yml up -d

# Check service health
docker-compose -f docker-compose.test.yml ps

# View logs
docker-compose -f docker-compose.test.yml logs -f
```

### Load Test Data

```bash
# PostgreSQL
docker-compose -f docker-compose.test.yml exec -T postgres \
  psql -U zipperfly -d zipperfly_test < test/fixtures/postgres_schema.sql

# MySQL
docker-compose -f docker-compose.test.yml exec -T mysql \
  mysql -u zipperfly -ptestpass zipperfly_test < test/fixtures/mysql_schema.sql

# MinIO - upload files
docker run --rm --network host \
  -v $(pwd)/test/fixtures/files:/data \
  -e MC_HOST_local=http://minioadmin:minioadmin@localhost:9000 \
  minio/mc \
  cp /data/document.txt local/test-bucket/document.txt
```

### Run Zipperfly Against Test Services

```bash
# PostgreSQL + Local Storage
export DB_URL="postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable"
export STORAGE_TYPE="local"
export STORAGE_PATH="$(pwd)/test/fixtures/files"
./zipperfly

# Then test:
curl http://localhost:8080/test-basic -o download.zip

# MySQL + Local Storage
export DB_URL="mysql://zipperfly:testpass@tcp(localhost:3306)/zipperfly_test"
./zipperfly

# PostgreSQL + MinIO (S3)
export DB_URL="postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable"
export STORAGE_TYPE="s3"
export S3_ENDPOINT="http://localhost:9000"
export S3_ACCESS_KEY_ID="minioadmin"
export S3_SECRET_ACCESS_KEY="minioadmin"
./zipperfly
```

## Integration Test Details

### Test Coverage

**Database Integration:**
- ✅ PostgreSQL connection and queries
- ✅ MySQL connection and queries
- ✅ Redis connection (basic smoke test)
- ✅ JSON field handling (objects, custom_headers)
- ✅ NULL field handling (password, callback)

**Storage Integration:**
- ✅ Local filesystem file fetching
- ✅ Multiple file downloads
- ✅ Binary file handling
- ✅ Missing file handling (IGNORE_MISSING=true)
- ⏳ MinIO/S3 integration (TODO)

**End-to-End:**
- ✅ Complete download flow (DB → Storage → ZIP → HTTP)
- ✅ ZIP file generation and validation
- ✅ File content verification
- ✅ HTTP response codes
- ⏳ Password-protected ZIPs (TODO)
- ⏳ Custom headers (TODO)
- ⏳ Callback execution (TODO)

### Running Specific Tests

```bash
# Run only PostgreSQL tests
go test -tags=integration ./test/integration -v -run TestIntegration_LocalStorage_PostgreSQL

# Run only MySQL tests
go test -tags=integration ./test/integration -v -run TestIntegration_LocalStorage_MySQL

# Run only Redis tests
go test -tags=integration ./test/integration -v -run TestIntegration_LocalStorage_Redis
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests
on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Start test services
        run: make test-integration-setup

      - name: Run integration tests
        run: make test-integration

      - name: Stop test services
        if: always()
        run: make test-integration-down
```

## Troubleshooting

### Services won't start

```bash
# Check Docker is running
docker ps

# Check ports aren't in use
lsof -i :5432  # PostgreSQL
lsof -i :3306  # MySQL
lsof -i :6379  # Redis
lsof -i :9000  # MinIO

# Force clean restart
docker-compose -f docker-compose.test.yml down -v
make test-integration-setup
```

### Tests fail with "service not available"

```bash
# Wait longer for services to be ready
sleep 10
make test-integration

# Check service logs
docker-compose -f docker-compose.test.yml logs postgres
docker-compose -f docker-compose.test.yml logs mysql
```

### Can't connect to database

```bash
# Test PostgreSQL directly
docker-compose -f docker-compose.test.yml exec postgres psql -U zipperfly -d zipperfly_test

# Test MySQL directly
docker-compose -f docker-compose.test.yml exec mysql mysql -u zipperfly -ptestpass zipperfly_test
```

## Adding New Tests

1. **Add test files** to `fixtures/files/`
2. **Add test records** to schema files
3. **Add test cases** to integration tests
4. **Update this README** with new test scenarios

Example:

```go
{
    name:          "new test scenario",
    downloadID:    "test-new",
    wantStatus:    http.StatusOK,
    wantFiles:     []string{"new-file.txt"},
    checkZipValid: true,
},
```
