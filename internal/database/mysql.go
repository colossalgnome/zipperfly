package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// MySQLStore implements Store for MySQL
type MySQLStore struct {
	db               *sql.DB
	tableName        string
	idField          string
	timeout          time.Duration
	metrics          *metrics.Metrics
	availableColumns map[string]bool // tracks which optional columns exist
}

// NewMySQLStore creates a new MySQL store
func NewMySQLStore(cfg *config.Config, m *metrics.Metrics) (*MySQLStore, error) {
	// Convert URL format to DSN format if needed
	dsn, err := mysqlURLtoDSN(cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("invalid mysql url: %w", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql connect error: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.DBMaxConnections)
	db.SetMaxIdleConns(min(2, cfg.DBMaxConnections)) // Keep a few connections idle (or max if max < 2)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	store := &MySQLStore{
		db:               db,
		tableName:        cfg.TableName,
		idField:          cfg.IDField,
		timeout:          cfg.DatabaseQueryTimeout,
		metrics:          m,
		availableColumns: make(map[string]bool),
	}

	// Detect which optional columns exist in the table
	ctx := context.Background()
	if err := store.detectColumns(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to detect table columns: %w", err)
	}

	return store, nil
}

// mysqlURLtoDSN converts mysql://user:pass@host:port/db to user:pass@tcp(host:port)/db
func mysqlURLtoDSN(urlStr string) (string, error) {
	// If it doesn't start with mysql://, assume it's already in DSN format
	if !strings.HasPrefix(urlStr, "mysql://") {
		return urlStr, nil
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Extract user:pass
	userInfo := ""
	if u.User != nil {
		if password, ok := u.User.Password(); ok {
			userInfo = fmt.Sprintf("%s:%s@", u.User.Username(), password)
		} else {
			userInfo = fmt.Sprintf("%s@", u.User.Username())
		}
	}

	// Extract host:port
	host := u.Host
	if host == "" {
		host = "localhost:3306"
	} else if !strings.Contains(host, ":") {
		host = host + ":3306"
	}

	// Extract database name
	dbName := strings.TrimPrefix(u.Path, "/")

	// Build DSN: user:pass@tcp(host:port)/dbname
	dsn := fmt.Sprintf("%stcp(%s)/%s", userInfo, host, dbName)

	// Append query parameters if any
	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}

	return dsn, nil
}

// detectColumns queries the database schema to determine which optional columns exist
func (s *MySQLStore) detectColumns(ctx context.Context) error {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = ? AND table_schema = DATABASE()
	`

	rows, err := s.db.QueryContext(ctx, query, s.tableName)
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
func (s *MySQLStore) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.metrics.DatabaseQueryDuration.WithLabelValues("mysql").Observe(duration.Seconds())
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
		"SELECT %s FROM %s WHERE %s = ?",
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
	err := s.db.QueryRowContext(queryCtx, query, id).Scan(scanDests...)
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
func (s *MySQLStore) Close() error {
	return s.db.Close()
}
