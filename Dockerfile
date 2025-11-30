# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o zipperfly \
    ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 zipperfly && \
    adduser -D -u 1000 -G zipperfly zipperfly

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/zipperfly .

# Create directory for Let's Encrypt certs (if needed)
RUN mkdir -p /app/certs && chown zipperfly:zipperfly /app/certs

# Switch to non-root user
USER zipperfly

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/bin/sh", "-c", "wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1"]

# Run the binary
ENTRYPOINT ["/app/zipperfly"]
