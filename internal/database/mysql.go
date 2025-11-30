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
	db        *sql.DB
	tableName string
	idField   string
	timeout   time.Duration
	metrics   *metrics.Metrics
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

	return &MySQLStore{
		db:        db,
		tableName: cfg.TableName,
		idField:   cfg.IDField,
		timeout:   cfg.DatabaseQueryTimeout,
		metrics:   m,
	}, nil
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
	var callbackVal, passwordVal, customHeadersJSON sql.NullString

	query := fmt.Sprintf(
		"SELECT bucket, objects, name, callback, password, custom_headers FROM %s WHERE %s = ?",
		s.tableName,
		s.idField,
	)

	err := s.db.QueryRowContext(queryCtx, query, id).Scan(
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
func (s *MySQLStore) Close() error {
	return s.db.Close()
}
