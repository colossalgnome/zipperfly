package config

import (
    "os"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue time.Duration
		want         time.Duration
	}{
		{
			name:         "empty string uses default",
			input:        "",
			defaultValue: 5 * time.Second,
			want:         5 * time.Second,
		},
		{
			name:         "valid duration",
			input:        "10s",
			defaultValue: 5 * time.Second,
			want:         10 * time.Second,
		},
		{
			name:         "minutes",
			input:        "5m",
			defaultValue: 1 * time.Second,
			want:         5 * time.Minute,
		},
		{
			name:         "invalid duration uses default",
			input:        "invalid",
			defaultValue: 3 * time.Second,
			want:         3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDuration(tt.input, tt.defaultValue)
			if got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}


func TestParseInt(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue int
		want         int
	}{
		{
			name:         "empty string uses default",
			input:        "",
			defaultValue: 10,
			want:         10,
		},
		{
			name:         "valid integer",
			input:        "42",
			defaultValue: 10,
			want:         42,
		},
		{
			name:         "zero",
			input:        "0",
			defaultValue: 10,
			want:         0,
		},
		{
			name:         "invalid input uses default",
			input:        "not-a-number",
			defaultValue: 5,
			want:         5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt(tt.input, tt.defaultValue)
			if got != tt.want {
				t.Errorf("parseInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single item",
			input: "foo",
			want:  []string{"foo"},
		},
		{
			name:  "multiple items",
			input: "foo,bar,baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "items with spaces",
			input: "foo, bar , baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "empty items filtered out",
			input: "foo,,bar, ,baz",
			want:  []string{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStringList(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseStringList() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseStringList()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoad_MissingDBURL_ReturnsError(t *testing.T) {
	t.Setenv("DB_URL", "")
	// Make sure no HTTPS envs accidentally trip validation
	t.Setenv("ENABLE_HTTPS", "false")

	if _, err := Load(); err == nil {
		t.Fatalf("expected error when DB_URL is missing, got nil")
	}
}

func TestLoad_EnableHTTPSMissingDomains_ReturnsError(t *testing.T) {
	t.Setenv("DB_URL", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	t.Setenv("ENABLE_HTTPS", "true")
	t.Setenv("LETSENCRYPT_DOMAINS", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected error when ENABLE_HTTPS=true and LETSENCRYPT_DOMAINS empty, got nil")
	}
}

func TestLoad_ValidConfig_WithHTTPSAndLocalStorage(t *testing.T) {
	// Clean slate
	for _, key := range []string{
		"DB_URL", "ENABLE_HTTPS", "LETSENCRYPT_DOMAINS",
		"STORAGE_TYPE", "STORAGE_PATH",
		"S3_REGION", "S3_FORCE_PATH_STYLE", "MAX_CONCURRENT_FETCHES",
	} {
		os.Unsetenv(key)
	}

	t.Setenv("DB_URL", "mysql://user:pass@localhost:3306/dbname")
	t.Setenv("ENABLE_HTTPS", "true")
	t.Setenv("LETSENCRYPT_DOMAINS", "example.com,example.org")
	t.Setenv("STORAGE_PATH", "/tmp/files")
	// Let STORAGE_TYPE auto-detect to "local"
	t.Setenv("MAX_CONCURRENT_FETCHES", "25")
	t.Setenv("DATABASE_QUERY_TIMEOUT", "10s")
	t.Setenv("STORAGE_FETCH_TIMEOUT", "30s")
	t.Setenv("REQUEST_TIMEOUT", "120s")
	t.Setenv("MAX_FILES_PER_REQUEST", "50")
	t.Setenv("STORAGE_MAX_RETRIES", "5")
	t.Setenv("STORAGE_RETRY_DELAY", "2s")
	t.Setenv("CIRCUIT_BREAKER_THRESHOLD", "3")
	t.Setenv("CIRCUIT_BREAKER_TIMEOUT", "5s")
	t.Setenv("CIRCUIT_BREAKER_MAX_REQUESTS", "4")
	t.Setenv("ALLOW_PASSWORD_PROTECTED", "true")
	t.Setenv("ALLOWED_EXTENSIONS", ".txt,.csv")
	t.Setenv("BLOCKED_EXTENSIONS", ".exe,.bat")
	t.Setenv("CALLBACK_MAX_RETRIES", "7")
	t.Setenv("CALLBACK_RETRY_DELAY", "9s")
	t.Setenv("PORT", "9090")
	t.Setenv("S3_REGION", "") // to hit default "auto"

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DBEngine != "mysql" {
		t.Errorf("expected DBEngine=mysql, got %q", cfg.DBEngine)
	}
	if cfg.StorageType != "local" {
		t.Errorf("expected StorageType=local (auto-detected), got %q", cfg.StorageType)
	}
	if cfg.StoragePath != "/tmp/files" {
		t.Errorf("expected StoragePath=/tmp/files, got %q", cfg.StoragePath)
	}
	if cfg.EnableHTTPS != true {
		t.Errorf("expected EnableHTTPS=true, got %v", cfg.EnableHTTPS)
	}
	if len(cfg.LetsEncryptDomains) != 2 {
		t.Fatalf("expected 2 LetsEncryptDomains, got %d", len(cfg.LetsEncryptDomains))
	}
	if cfg.LetsEncryptDomains[0] != "example.com" || cfg.LetsEncryptDomains[1] != "example.org" {
		t.Errorf("unexpected LetsEncryptDomains: %#v", cfg.LetsEncryptDomains)
	}
	if cfg.S3Region != "auto" {
		t.Errorf("expected S3Region default 'auto', got %q", cfg.S3Region)
	}
    if cfg.S3UsePathStyle != false {
		t.Errorf("expected S3UsePathStyle default false, got %v", cfg.S3UsePathStyle)
	}
	if cfg.MaxConcurrent != 25 {
		t.Errorf("expected MaxConcurrent=25, got %d", cfg.MaxConcurrent)
	}
	if cfg.DatabaseQueryTimeout != 10*time.Second {
		t.Errorf("unexpected DatabaseQueryTimeout: %v", cfg.DatabaseQueryTimeout)
	}
	if cfg.StorageFetchTimeout != 30*time.Second {
		t.Errorf("unexpected StorageFetchTimeout: %v", cfg.StorageFetchTimeout)
	}
	if cfg.RequestTimeout != 120*time.Second {
		t.Errorf("unexpected RequestTimeout: %v", cfg.RequestTimeout)
	}
	if cfg.MaxFilesPerRequest != 50 {
		t.Errorf("expected MaxFilesPerRequest=50, got %d", cfg.MaxFilesPerRequest)
	}
	if cfg.StorageMaxRetries != 5 {
		t.Errorf("expected StorageMaxRetries=5, got %d", cfg.StorageMaxRetries)
	}
	if cfg.StorageRetryDelay != 2*time.Second {
		t.Errorf("expected StorageRetryDelay=2s, got %v", cfg.StorageRetryDelay)
	}
	if cfg.CircuitBreakerThreshold != 3 {
		t.Errorf("expected CircuitBreakerThreshold=3, got %d", cfg.CircuitBreakerThreshold)
	}
	if cfg.CircuitBreakerTimeout != 5*time.Second {
		t.Errorf("expected CircuitBreakerTimeout=5s, got %v", cfg.CircuitBreakerTimeout)
	}
	if cfg.CircuitBreakerMaxRequests != 4 {
		t.Errorf("expected CircuitBreakerMaxRequests=4, got %d", cfg.CircuitBreakerMaxRequests)
	}
	if !cfg.AllowPasswordProtected {
		t.Errorf("expected AllowPasswordProtected=true")
	}
	if len(cfg.AllowedExtensions) != 2 || cfg.AllowedExtensions[0] != ".txt" {
		t.Errorf("unexpected AllowedExtensions: %#v", cfg.AllowedExtensions)
	}
	if len(cfg.BlockedExtensions) != 2 || cfg.BlockedExtensions[0] != ".exe" {
		t.Errorf("unexpected BlockedExtensions: %#v", cfg.BlockedExtensions)
	}
	if cfg.CallbackMaxRetries != 7 {
		t.Errorf("expected CallbackMaxRetries=7, got %d", cfg.CallbackMaxRetries)
	}
	if cfg.CallbackRetryDelay != 9*time.Second {
		t.Errorf("expected CallbackRetryDelay=9s, got %v", cfg.CallbackRetryDelay)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected Port=9090, got %s", cfg.Port)
	}
}

func TestParseHelpers(t *testing.T) {
	// parseDuration
	if got := parseDuration("2s", 5*time.Second); got != 2*time.Second {
		t.Errorf("parseDuration valid: expected 2s, got %v", got)
	}
	if got := parseDuration("not-a-duration", 5*time.Second); got != 5*time.Second {
		t.Errorf("parseDuration invalid: expected default 5s, got %v", got)
	}

	// parseInt
	if got := parseInt("42", 5); got != 42 {
		t.Errorf("parseInt valid expected 42, got %d", got)
	}
	if got := parseInt("nope", 7); got != 7 {
		t.Errorf("parseInt invalid expected default 7, got %d", got)
	}

	// parseStringList
	list := parseStringList("  a , , b, c  ")
	if len(list) != 3 || list[0] != "a" || list[1] != "b" || list[2] != "c" {
		t.Errorf("parseStringList unexpected result: %#v", list)
	}
	if list := parseStringList(""); list != nil {
		t.Errorf("expected nil for empty string list, got %#v", list)
	}
}
