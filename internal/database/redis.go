package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// RedisStore implements Store for Redis
type RedisStore struct {
	client    *redis.Client
	keyPrefix string
	timeout   time.Duration
	metrics   *metrics.Metrics
}

// NewRedisStore creates a new Redis store
func NewRedisStore(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (*RedisStore, error) {
	opts, err := redis.ParseURL(cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url error: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connect error: %w", err)
	}

	return &RedisStore{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
		timeout:   cfg.DatabaseQueryTimeout,
		metrics:   m,
	}, nil
}

// GetRecord retrieves a download record by ID
func (s *RedisStore) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.metrics.DatabaseQueryDuration.WithLabelValues("redis").Observe(duration.Seconds())
	}()

	// Apply timeout
	queryCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var record models.DownloadRecord

	data, err := s.client.Get(queryCtx, s.keyPrefix+id).Bytes()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	record.ID = id
	return &record, nil
}

// Close closes the Redis connection
func (s *RedisStore) Close() error {
	return s.client.Close()
}
