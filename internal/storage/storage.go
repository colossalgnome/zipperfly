package storage

import (
	"context"
	"fmt"
	"io"

	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

// Provider defines the interface for storage backends
type Provider interface {
	// GetObject retrieves an object from storage
	// bucket: the bucket name (S3) or base path (local)
	// key: the object key/path
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// HealthCheck performs a lightweight connectivity check
	HealthCheck(ctx context.Context) error
}

// New creates a new storage provider based on configuration
func New(ctx context.Context, cfg *config.Config, m *metrics.Metrics, cb *circuitbreaker.Breaker) (Provider, error) {
	switch cfg.StorageType {
	case "s3":
		return NewS3Provider(ctx, cfg, m, cb)
	case "local":
		if cfg.StoragePath == "" {
			return nil, fmt.Errorf("STORAGE_PATH required for local storage")
		}
		return NewLocalProvider(cfg.StoragePath, m, cb, cfg.StorageFetchTimeout, cfg.StorageMaxRetries, cfg.StorageRetryDelay)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.StorageType)
	}
}
