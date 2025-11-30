package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Database
	DBURL            string
	DBEngine         string
	DBMaxConnections int // connection pool size (default: 20)
	TableName        string
	IDField          string
	KeyPrefix        string // For Redis

	// Storage
	StorageType       string // "s3" or "local"
	StoragePath       string // For local filesystem storage

	// S3
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool

	// Security
	EnforceSigning bool
	SigningSecret  []byte

	// Timeouts (in seconds)
	DatabaseQueryTimeout time.Duration
	StorageFetchTimeout  time.Duration
	RequestTimeout       time.Duration

	// Resource Limits
	MaxActiveDownloads int     // max concurrent downloads, 0 = unlimited
	MaxFilesPerRequest int     // max files per download, 0 = unlimited
	RateLimitPerIP     float64 // requests per second per IP, 0 = unlimited

	// Retries
	StorageMaxRetries int
	StorageRetryDelay time.Duration

	// Circuit Breaker
	CircuitBreakerThreshold   int           // failures before opening
	CircuitBreakerTimeout     time.Duration // time to wait before half-open
	CircuitBreakerMaxRequests int           // max requests in half-open state

	// Features
	AppendYMD             bool
	SanitizeNames         bool
	IgnoreMissing         bool
	MaxConcurrent         int64
	AllowPasswordProtected bool

	// File Filtering
	AllowedExtensions []string // empty = allow all
	BlockedExtensions []string

	// Callback
	CallbackMaxRetries int
	CallbackRetryDelay time.Duration

	// Server
	Port        string
	EnableHTTPS bool

	// Let's Encrypt
	LetsEncryptDomains  []string
	LetsEncryptCacheDir string
	LetsEncryptEmail    string

	// Metrics
	MetricsUsername string
	MetricsPassword string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL required")
	}

	u, err := url.Parse(dbURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_URL: %w", err)
	}

	maxConcurrentStr := os.Getenv("MAX_CONCURRENT_FETCHES")
	maxConcurrent := int64(10) // default
	if maxConcurrentStr != "" {
		maxConcurrent, err = strconv.ParseInt(maxConcurrentStr, 10, 64)
		if err != nil || maxConcurrent < 1 {
			return nil, fmt.Errorf("invalid MAX_CONCURRENT_FETCHES: %w", err)
		}
	}

	enforceSigning, _ := strconv.ParseBool(os.Getenv("ENFORCE_SIGNING"))
	appendYMD, _ := strconv.ParseBool(os.Getenv("APPEND_YMD"))
	sanitizeNames, _ := strconv.ParseBool(os.Getenv("SANITIZE_FILENAMES"))
	ignoreMissing, _ := strconv.ParseBool(os.Getenv("IGNORE_MISSING"))
	enableHTTPS, _ := strconv.ParseBool(os.Getenv("ENABLE_HTTPS"))

	idField := os.Getenv("ID_FIELD")
	if idField == "" {
		idField = "id"
	}

	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		tableName = "downloads"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s3Region := os.Getenv("S3_REGION")
	if s3Region == "" {
		s3Region = "auto"
	}

    s3UsePathStyle := false
	if v := os.Getenv("S3_USE_PATH_STYLE"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			s3UsePathStyle = parsed
		}
	}

	var letsEncryptDomains []string
	if enableHTTPS {
		domains := strings.Split(os.Getenv("LETSENCRYPT_DOMAINS"), ",")
		if len(domains) == 0 || domains[0] == "" {
			return nil, fmt.Errorf("LETSENCRYPT_DOMAINS required when ENABLE_HTTPS=true")
		}
		letsEncryptDomains = domains
	}

	letsEncryptCacheDir := os.Getenv("LETSENCRYPT_CACHE_DIR")
	if letsEncryptCacheDir == "" {
		letsEncryptCacheDir = "./certs"
	}

	// Determine storage type
	storageType := os.Getenv("STORAGE_TYPE")
	storagePath := os.Getenv("STORAGE_PATH")

	// Auto-detect storage type if not specified
	if storageType == "" {
		if storagePath != "" {
			storageType = "local"
		} else {
			storageType = "s3"
		}
	}

	// Parse database settings
	dbMaxConnections := parseInt(os.Getenv("DB_MAX_CONNECTIONS"), 20)

	// Parse timeouts
	dbTimeout := parseDuration(os.Getenv("DATABASE_QUERY_TIMEOUT"), 5*time.Second)
	storageTimeout := parseDuration(os.Getenv("STORAGE_FETCH_TIMEOUT"), 60*time.Second)
	requestTimeout := parseDuration(os.Getenv("REQUEST_TIMEOUT"), 300*time.Second)

	// Parse resource limits
	maxActiveDownloads := parseInt(os.Getenv("MAX_ACTIVE_DOWNLOADS"), 0)
	maxFilesPerRequest := parseInt(os.Getenv("MAX_FILES_PER_REQUEST"), 0)
	rateLimitPerIP := parseFloat(os.Getenv("RATE_LIMIT_PER_IP"), 0)

	// Parse retry settings
	storageMaxRetries := parseInt(os.Getenv("STORAGE_MAX_RETRIES"), 3)
	storageRetryDelay := parseDuration(os.Getenv("STORAGE_RETRY_DELAY"), 1*time.Second)

	// Parse circuit breaker settings
	cbThreshold := parseInt(os.Getenv("CIRCUIT_BREAKER_THRESHOLD"), 5)
	cbTimeout := parseDuration(os.Getenv("CIRCUIT_BREAKER_TIMEOUT"), 60*time.Second)
	cbMaxRequests := parseInt(os.Getenv("CIRCUIT_BREAKER_MAX_REQUESTS"), 2)

	// Parse feature flags
	allowPasswordProtected, _ := strconv.ParseBool(os.Getenv("ALLOW_PASSWORD_PROTECTED"))

	// Parse file extension filters
	allowedExts := parseStringList(os.Getenv("ALLOWED_EXTENSIONS"))
	blockedExts := parseStringList(os.Getenv("BLOCKED_EXTENSIONS"))

	// Parse callback settings
	callbackMaxRetries := parseInt(os.Getenv("CALLBACK_MAX_RETRIES"), 3)
	callbackRetryDelay := parseDuration(os.Getenv("CALLBACK_RETRY_DELAY"), 5*time.Second)

	return &Config{
		DBURL:            dbURL,
		DBEngine:         u.Scheme,
		DBMaxConnections: dbMaxConnections,
		TableName:        tableName,
		IDField:          idField,
		KeyPrefix:        os.Getenv("KEY_PREFIX"),
		StorageType:         storageType,
		StoragePath:         storagePath,
		S3Endpoint:          os.Getenv("S3_ENDPOINT"),
		S3Region:            s3Region,
		S3AccessKeyID:       os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey:   os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3UsePathStyle:      s3UsePathStyle,
		EnforceSigning:      enforceSigning,
		SigningSecret:       []byte(os.Getenv("SIGNING_SECRET")),
		DatabaseQueryTimeout: dbTimeout,
		StorageFetchTimeout:  storageTimeout,
		RequestTimeout:       requestTimeout,
		MaxActiveDownloads:   maxActiveDownloads,
		MaxFilesPerRequest:   maxFilesPerRequest,
		RateLimitPerIP:       rateLimitPerIP,
		StorageMaxRetries:    storageMaxRetries,
		StorageRetryDelay:    storageRetryDelay,
		CircuitBreakerThreshold:   cbThreshold,
		CircuitBreakerTimeout:     cbTimeout,
		CircuitBreakerMaxRequests: cbMaxRequests,
		AppendYMD:             appendYMD,
		SanitizeNames:         sanitizeNames,
		IgnoreMissing:         ignoreMissing,
		MaxConcurrent:         maxConcurrent,
		AllowPasswordProtected: allowPasswordProtected,
		AllowedExtensions:     allowedExts,
		BlockedExtensions:     blockedExts,
		CallbackMaxRetries:    callbackMaxRetries,
		CallbackRetryDelay:    callbackRetryDelay,
		Port:                  port,
		EnableHTTPS:           enableHTTPS,
		LetsEncryptDomains:    letsEncryptDomains,
		LetsEncryptCacheDir:   letsEncryptCacheDir,
		LetsEncryptEmail:      os.Getenv("LETSENCRYPT_EMAIL"),
		MetricsUsername:       os.Getenv("METRICS_USERNAME"),
		MetricsPassword:       os.Getenv("METRICS_PASSWORD"),
	}, nil
}

// Helper functions for parsing configuration values

func parseDuration(s string, defaultValue time.Duration) time.Duration {
	if s == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultValue
	}
	return d
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}

func parseFloat(s string, defaultValue float64) float64 {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}
	return val
}

func parseStringList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
