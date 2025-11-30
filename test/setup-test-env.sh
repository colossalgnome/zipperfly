#!/bin/bash
set -e

echo "Setting up integration test environment..."

# Start Docker containers
echo "Starting Docker containers..."
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 5

# Check PostgreSQL
echo "Setting up PostgreSQL..."
until docker-compose -f docker-compose.test.yml exec -T postgres pg_isready -U zipperfly > /dev/null 2>&1; do
  echo "Waiting for PostgreSQL..."
  sleep 2
done

# Load PostgreSQL schema
docker-compose -f docker-compose.test.yml exec -T postgres psql -U zipperfly -d zipperfly_test < fixtures/postgres_schema.sql
echo "✓ PostgreSQL ready"

# Check MySQL
echo "Setting up MySQL..."
until docker-compose -f docker-compose.test.yml exec -T mysql mysqladmin ping -h localhost -u zipperfly -ptestpass --silent > /dev/null 2>&1; do
  echo "Waiting for MySQL..."
  sleep 2
done

# Load MySQL schema
docker-compose -f docker-compose.test.yml exec -T mysql mysql -u zipperfly -ptestpass zipperfly_test < fixtures/mysql_schema.sql
echo "✓ MySQL ready"

# Check Redis
echo "Checking Redis..."
until docker-compose -f docker-compose.test.yml exec -T redis redis-cli ping > /dev/null 2>&1; do
  echo "Waiting for Redis..."
  sleep 2
done
echo "✓ Redis ready"

# Check MinIO
echo "Checking MinIO..."
until curl -f http://localhost:9000/minio/health/live > /dev/null 2>&1; do
  echo "Waiting for MinIO..."
  sleep 2
done
echo "✓ MinIO ready (test files mounted at /data/test-bucket)"

echo ""
echo "✅ All services ready for testing!"
echo ""
echo "Run integration tests with:"
echo "  make test-integration"
echo ""
echo "Or manually:"
echo "  go test -tags=integration ./test/integration -v"
echo ""
echo "Stop test environment:"
echo "  docker-compose -f docker-compose.test.yml down"
