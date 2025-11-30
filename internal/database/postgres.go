package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// PostgresStore implements Store for PostgreSQL
type PostgresStore struct {
	pool      *pgxpool.Pool
	tableName string
	idField   string
	timeout   time.Duration
	metrics   *metrics.Metrics
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("postgres connect error: %w", err)
	}

	return &PostgresStore{
		pool:      pool,
		tableName: cfg.TableName,
		idField:   cfg.IDField,
		timeout:   cfg.DatabaseQueryTimeout,
		metrics:   m,
	}, nil
}

// GetRecord retrieves a download record by ID
func (s *PostgresStore) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.metrics.DatabaseQueryDuration.WithLabelValues("postgres").Observe(duration.Seconds())
	}()

	// Apply timeout
	queryCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var record models.DownloadRecord
	var objectsJSON []byte
	var callbackVal, passwordVal, customHeadersJSON sql.NullString

	query := fmt.Sprintf(
		"SELECT bucket, objects, name, callback, password, custom_headers FROM %s WHERE %s = $1",
		s.tableName,
		s.idField,
	)

	err := s.pool.QueryRow(queryCtx, query, id).Scan(
		&record.Bucket,
		&objectsJSON,
		&record.Name,
		&callbackVal,
		&passwordVal,
		&customHeadersJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(objectsJSON, &record.Objects); err != nil {
		return nil, err
	}

	if callbackVal.Valid {
		record.Callback = callbackVal.String
	}

	if passwordVal.Valid {
		record.Password = passwordVal.String
	}

	if customHeadersJSON.Valid && customHeadersJSON.String != "" {
		if err := json.Unmarshal([]byte(customHeadersJSON.String), &record.CustomHeaders); err != nil {
			return nil, err
		}
	}

	record.ID = id
	return &record, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}
