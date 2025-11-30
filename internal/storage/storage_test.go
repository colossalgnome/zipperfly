package storage

import (
	"context"
	"testing"
	"time"

	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

func TestNew_LocalStorage(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	cfg := &config.Config{
		StorageType:               "local",
		StoragePath:               tmpDir,
		StorageFetchTimeout:       5 * time.Second,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := New(ctx, cfg, m, cb)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if provider == nil {
		t.Fatal("New() returned nil provider")
	}

	// Verify it's a LocalProvider
	if _, ok := provider.(*LocalProvider); !ok {
		t.Errorf("expected *LocalProvider, got %T", provider)
	}
}

func TestNew_LocalStorage_MissingPath(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		StorageType:               "local",
		StoragePath:               "", // Missing path
		StorageFetchTimeout:       5 * time.Second,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := New(ctx, cfg, m, cb)
	if err == nil {
		t.Error("New() should return error for local storage without STORAGE_PATH")
	}

	if provider != nil {
		t.Error("New() should return nil provider on error")
	}

	expectedErr := "STORAGE_PATH required for local storage"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestNew_S3Storage(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		StorageType:               "s3",
		S3Endpoint:                "http://localhost:9000",
		S3Region:                  "us-east-1",
		S3AccessKeyID:             "test-key",
		S3SecretAccessKey:         "test-secret",
		S3UsePathStyle:            true,
		StorageFetchTimeout:       5 * time.Second,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := New(ctx, cfg, m, cb)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if provider == nil {
		t.Fatal("New() returned nil provider")
	}

	// Verify it's an S3Provider
	if _, ok := provider.(*S3Provider); !ok {
		t.Errorf("expected *S3Provider, got %T", provider)
	}
}

func TestNew_UnsupportedStorageType(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		StorageType:               "unsupported-type",
		StorageFetchTimeout:       5 * time.Second,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
	}

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := New(ctx, cfg, m, cb)
	if err == nil {
		t.Error("New() should return error for unsupported storage type")
	}

	if provider != nil {
		t.Error("New() should return nil provider on error")
	}

	expectedErr := "unsupported storage type: unsupported-type"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("error = %q, want %q", err.Error(), expectedErr)
	}
}
