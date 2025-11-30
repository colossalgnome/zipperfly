package storage

import (
	"context"
	"testing"
	"time"

	appconfig "zipperfly/internal/config"
	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/metrics"
)

func baseS3TestConfig() *appconfig.Config {
	return &appconfig.Config{
		S3Endpoint:              "http://example.com", // we won't actually call it
		S3Region:                "us-east-1",
		S3AccessKeyID:           "test-access-key",
		S3SecretAccessKey:       "test-secret-key",
		S3UsePathStyle:          true, // default; individual tests will override
		StorageFetchTimeout:     2 * time.Second,
		StorageMaxRetries:       1,
		StorageRetryDelay:       10 * time.Millisecond,
		CircuitBreakerThreshold: 1,
		CircuitBreakerTimeout:   time.Second,
		CircuitBreakerMaxRequests: 1,
	}
}

func TestNewS3Provider_UsePathStyleTrue(t *testing.T) {
	ctx := context.Background()
	cfg := baseS3TestConfig()
	cfg.S3UsePathStyle = true

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := NewS3Provider(ctx, cfg, m, cb)
	if err != nil {
		t.Fatalf("NewS3Provider returned error: %v", err)
	}
	if provider == nil || provider.client == nil {
		t.Fatalf("NewS3Provider returned nil provider or client")
	}

	opts := provider.client.Options()
	if !opts.UsePathStyle {
		t.Errorf("expected UsePathStyle=true on s3 client options when cfg.S3UsePathStyle=true")
	}
}

func TestNewS3Provider_UsePathStyleFalse(t *testing.T) {
	ctx := context.Background()
	cfg := baseS3TestConfig()
	cfg.S3UsePathStyle = false

	m := metrics.New()
	cb := circuitbreaker.New("storage", cfg, m)

	provider, err := NewS3Provider(ctx, cfg, m, cb)
	if err != nil {
		t.Fatalf("NewS3Provider returned error: %v", err)
	}
	if provider == nil || provider.client == nil {
		t.Fatalf("NewS3Provider returned nil provider or client")
	}

	opts := provider.client.Options()
	if opts.UsePathStyle {
		t.Errorf("expected UsePathStyle=false on s3 client options when cfg.S3UsePathStyle=false")
	}
}
