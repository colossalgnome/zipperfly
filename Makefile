.PHONY: build test test-coverage test-verbose test-integration test-integration-setup test-integration-down clean run

# Build the application
build:
	go build -o zipperfly ./cmd/server

# Run unit tests only
test:
	go test -short ./...

# Run all tests (unit + integration)
test-all:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -short ./... -cover

# Run tests with verbose output
test-verbose:
	go test -short ./... -v

# Run tests with coverage report and generate HTML
test-coverage-html:
	go test -short ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Setup integration test environment (Docker)
test-integration-setup:
	@echo "Setting up integration test environment..."
	./test/setup-test-env.sh

# Run integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	@echo "Make sure test environment is running (make test-integration-setup)"
	go test -tags=integration ./test/integration -v

# Run integration tests with setup
test-integration-full: test-integration-setup
	@sleep 2
	$(MAKE) test-integration

# Stop integration test environment
test-integration-down:
	docker-compose -f docker-compose.test.yml down -v

# Clean build artifacts
clean:
	rm -f zipperfly coverage.out coverage.html

# Run the application (requires configuration)
run: build
	./zipperfly

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Run all checks (format, lint, unit tests)
check: fmt test
	@echo "All checks passed!"

# Full CI pipeline (unit + integration)
ci: check test-integration-full
	@echo "CI pipeline complete!"
