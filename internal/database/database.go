package database

import (
	"context"
	"fmt"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// Store defines the interface for database operations
type Store interface {
	GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error)
	Close() error
}

// These indirection variables allow tests to override the concrete
// store constructors so we can exercise New(...) without real DBs.
var (
	newPostgresStoreFunc = func(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (Store, error) {
		return NewPostgresStore(ctx, cfg, m)
	}
	newMySQLStoreFunc = func(cfg *config.Config, m *metrics.Metrics) (Store, error) {
		return NewMySQLStore(cfg, m)
	}
	newRedisStoreFunc = func(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (Store, error) {
		return NewRedisStore(ctx, cfg, m)
	}
)

// New creates a new database store based on the configured engine
func New(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (Store, error) {
	switch cfg.DBEngine {
	case "postgres", "postgresql":
		return newPostgresStoreFunc(ctx, cfg, m)
	case "mysql":
		return newMySQLStoreFunc(cfg, m)
	case "redis":
		return newRedisStoreFunc(ctx, cfg, m)
	default:
		return nil, fmt.Errorf("unsupported database engine: %s", cfg.DBEngine)
	}
}
