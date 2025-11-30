package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

// Shared metrics instance to avoid duplicate registration
var sharedMetrics = metrics.New()

func TestLocalProvider_GetObject(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(subFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create subdir file: %v", err)
	}

	// Setup provider
	cfg := &config.Config{
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}
	cb := circuitbreaker.New("test-storage", cfg, sharedMetrics)

	provider, err := NewLocalProvider(tmpDir, sharedMetrics, cb, 5*time.Second, 3, time.Second)
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}

	tests := []struct {
		name      string
		bucket    string
		key       string
		wantErr   bool
		errContains string
	}{
		{
			name:    "fetch existing file",
			bucket:  "",
			key:     "test.txt",
			wantErr: false,
		},
		{
			name:    "fetch file in subdirectory with bucket",
			bucket:  "subdir",
			key:     "file.txt",
			wantErr: false,
		},
		{
			name:      "file not found",
			bucket:    "",
			key:       "nonexistent.txt",
			wantErr:   true,
			errContains: "no such file",
		},
		{
			name:      "path traversal attempt",
			bucket:    "",
			key:       "../../../etc/passwd",
			wantErr:   true,
			errContains: "path traversal",
		},
		{
			name:      "path traversal with bucket",
			bucket:    "../../../",
			key:       "etc/passwd",
			wantErr:   true,
			errContains: "path traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader, err := provider.GetObject(ctx, tt.bucket, tt.key)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetObject() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetObject() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("GetObject() unexpected error = %v", err)
					return
				}
				if reader != nil {
					reader.Close()
				}
			}
		})
	}
}

func TestLocalProvider_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}
	cb := circuitbreaker.New("test-storage-health", cfg, sharedMetrics)

	provider, err := NewLocalProvider(tmpDir, sharedMetrics, cb, 5*time.Second, 3, time.Second)
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}

	t.Run("healthy when base path exists", func(t *testing.T) {
		err := provider.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("HealthCheck() error = %v, want nil", err)
		}
	})
}

func TestNewLocalProvider_InvalidPath(t *testing.T) {
	cfg := &config.Config{
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}
	cb := circuitbreaker.New("test-storage-invalid", cfg, sharedMetrics)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "nonexistent path",
			path:    "/nonexistent/path/that/does/not/exist",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLocalProvider(tt.path, sharedMetrics, cb, 5*time.Second, 3, time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLocalProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
