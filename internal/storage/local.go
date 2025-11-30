package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/metrics"
)

// LocalProvider implements Provider for local filesystem storage
type LocalProvider struct {
	basePath       string
	circuitBreaker *circuitbreaker.Breaker
	metrics        *metrics.Metrics
	fetchTimeout   time.Duration
	maxRetries     int
	retryDelay     time.Duration
}

// NewLocalProvider creates a new local filesystem storage provider
func NewLocalProvider(basePath string, m *metrics.Metrics, cb *circuitbreaker.Breaker, fetchTimeout time.Duration, maxRetries int, retryDelay time.Duration) (*LocalProvider, error) {
	// Ensure base path exists and is a directory
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("base path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path is not a directory: %s", basePath)
	}

	// Get absolute path for security checks
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base path: %w", err)
	}

	return &LocalProvider{
		basePath:       absPath,
		circuitBreaker: cb,
		metrics:        m,
		fetchTimeout:   fetchTimeout,
		maxRetries:     maxRetries,
		retryDelay:     retryDelay,
	}, nil
}

// GetObject retrieves a file from the local filesystem
// bucket: optional path prefix within basePath (can be empty)
// key: file path relative to bucket (or basePath if bucket is empty)
func (l *LocalProvider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	start := time.Now()
	var resultLabel string
	defer func() {
		duration := time.Since(start)
		l.metrics.StorageFetchDuration.WithLabelValues("local", resultLabel).Observe(duration.Seconds())
	}()

	// Track active file fetches
	l.metrics.ActiveFileFetches.Inc()
	defer l.metrics.ActiveFileFetches.Dec()

	// Execute with circuit breaker
	result, err := l.circuitBreaker.Execute(func() (interface{}, error) {
		// Build the full path - bucket is optional and treated as a prefix
		pathComponents := []string{l.basePath}

		if bucket != "" {
			// Split bucket by / to handle paths like "foo/bar/baz"
			pathComponents = append(pathComponents, bucket)
		}

		pathComponents = append(pathComponents, key)
		fullPath := filepath.Join(pathComponents...)

		// Clean the path to resolve any .. or . segments
		fullPath = filepath.Clean(fullPath)

		// Security: ensure the resolved path is still within basePath
		if !strings.HasPrefix(fullPath, l.basePath) {
			resultLabel = "error"
			return nil, fmt.Errorf("path traversal attempt detected: bucket=%s, key=%s", bucket, key)
		}

		// Retry loop with exponential backoff
		var lastErr error
		for attempt := 0; attempt <= l.maxRetries; attempt++ {
			if attempt > 0 {
				// Exponential backoff: retryDelay * 2^(attempt-1)
				delay := l.retryDelay * time.Duration(1<<(attempt-1))
				time.Sleep(delay)
			}

			// Check context cancellation
			select {
			case <-ctx.Done():
				resultLabel = "error"
				return nil, ctx.Err()
			default:
			}

			// Open the file
			file, err := os.Open(fullPath)
			if err == nil {
				resultLabel = "success"
				return file, nil
			}

			lastErr = err

			// Check if error is retryable
			if !isLocalRetryableError(err) || attempt == l.maxRetries {
				break
			}
		}

		resultLabel = "error"
		return nil, fmt.Errorf("failed to open file: %w", lastErr)
	})

	if err != nil {
		return nil, err
	}

	return result.(io.ReadCloser), nil
}

// isLocalRetryableError determines if a local filesystem error should trigger a retry
func isLocalRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	// File not found is not retryable
	if os.IsNotExist(err) {
		return false
	}

	// Permission errors are not retryable
	if os.IsPermission(err) {
		return false
	}

	// Temporary errors (network filesystem issues) are retryable
	// Most other errors (like I/O errors) might be transient
	return true
}

// HealthCheck verifies the base path is still accessible
func (l *LocalProvider) HealthCheck(ctx context.Context) error {
	// Stat the base path to ensure mount is still accessible
	_, err := os.Stat(l.basePath)
	if err != nil {
		return fmt.Errorf("base path unavailable: %w", err)
	}
	return nil
}
