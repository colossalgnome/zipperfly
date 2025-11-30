package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// PostgresStore implements Store for PostgreSQL
type PostgresStore struct {
	pool             *pgxpool.Pool
	tableName        string
	idField          string
	timeout          time.Duration
	metrics          *metrics.Metrics
	availableColumns map[string]bool // tracks which optional columns exist
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (*PostgresStore, error) {
	// Parse config and set connection pool parameters
	poolConfig, err := pgxpool.ParseConfig(cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("postgres parse config error: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = int32(cfg.DBMaxConnections)
	poolConfig.MinConns = int32(min(2, cfg.DBMaxConnections)) // Keep a few connections warm (or max if max < 2)
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("postgres connect error: %w", err)
	}

	store := &PostgresStore{
		pool:             pool,
		tableName:        cfg.TableName,
		idField:          cfg.IDField,
		timeout:          cfg.DatabaseQueryTimeout,
		metrics:          m,
		availableColumns: make(map[string]bool),
	}

	// Detect which optional columns exist in the table
	if err := store.detectColumns(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to detect table columns: %w", err)
	}

	return store, nil
}

// detectColumns queries the database schema to determine which optional columns exist
func (s *PostgresStore) detectColumns(ctx context.Context) error {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = $1
	`

	rows, err := s.pool.Query(ctx, query, s.tableName)
	if err != nil {
		return fmt.Errorf("failed to query table schema: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return fmt.Errorf("failed to scan column name: %w", err)
		}
		columns[colName] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating columns: %w", err)
	}

	// Check for required columns
	if !columns[s.idField] {
		return fmt.Errorf("required column %q not found in table %q", s.idField, s.tableName)
	}
	if !columns["bucket"] {
		return fmt.Errorf("required column 'bucket' not found in table %q", s.tableName)
	}
	if !columns["objects"] {
		return fmt.Errorf("required column 'objects' not found in table %q", s.tableName)
	}

	// Track optional columns
	s.availableColumns["name"] = columns["name"]
	s.availableColumns["callback"] = columns["callback"]
	s.availableColumns["password"] = columns["password"]
	s.availableColumns["custom_headers"] = columns["custom_headers"]

	return nil
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

	// Build dynamic SELECT query based on available columns
	selectCols := []string{"bucket", "objects"}
	if s.availableColumns["name"] {
		selectCols = append(selectCols, "name")
	}
	if s.availableColumns["callback"] {
		selectCols = append(selectCols, "callback")
	}
	if s.availableColumns["password"] {
		selectCols = append(selectCols, "password")
	}
	if s.availableColumns["custom_headers"] {
		selectCols = append(selectCols, "custom_headers")
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1",
		strings.Join(selectCols, ", "),
		s.tableName,
		s.idField,
	)

	// Prepare scan destinations based on available columns
	scanDests := []interface{}{&record.Bucket, &objectsJSON}

	var nameVal, callbackVal, passwordVal, customHeadersJSON sql.NullString
	if s.availableColumns["name"] {
		scanDests = append(scanDests, &nameVal)
	}
	if s.availableColumns["callback"] {
		scanDests = append(scanDests, &callbackVal)
	}
	if s.availableColumns["password"] {
		scanDests = append(scanDests, &passwordVal)
	}
	if s.availableColumns["custom_headers"] {
		scanDests = append(scanDests, &customHeadersJSON)
	}

	// Execute query
	err := s.pool.QueryRow(queryCtx, query, id).Scan(scanDests...)
	if err != nil {
		return nil, err
	}

	// Parse required fields
	if err := json.Unmarshal(objectsJSON, &record.Objects); err != nil {
		return nil, err
	}

	// Parse optional fields if they exist
	if s.availableColumns["name"] && nameVal.Valid {
		record.Name = nameVal.String
	}

	if s.availableColumns["callback"] && callbackVal.Valid {
		record.Callback = callbackVal.String
	}

	if s.availableColumns["password"] && passwordVal.Valid {
		record.Password = passwordVal.String
	}

	if s.availableColumns["custom_headers"] && customHeadersJSON.Valid && customHeadersJSON.String != "" {
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
