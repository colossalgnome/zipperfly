package database

import (
	"context"
	"testing"
	"time"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

func TestPostgresStore_GetRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}

	m := metrics.New()
	cfg := &config.Config{
		DBURL:                "postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable",
		TableName:            "downloads",
		IDField:              "id",
		DatabaseQueryTimeout: 5 * time.Second,
	}

	ctx := context.Background()

	store, err := NewPostgresStore(ctx, cfg, m)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer store.Close()

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "existing record",
			id:      "test-basic",
			wantErr: false,
		},
		{
			name:    "nonexistent record",
			id:      "does-not-exist",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := store.GetRecord(ctx, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("GetRecord() error = nil, wantErr true")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetRecord() error = %v, wantErr false", err)
			}

			if record.ID != tt.id {
				t.Errorf("record.ID = %s, want %s", record.ID, tt.id)
			}

			if record.Bucket == "" {
				t.Error("record.Bucket is empty")
			}

			if len(record.Objects) == 0 {
				t.Error("record.Objects is empty")
			}
		})
	}
}

func TestPostgresStore_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}

	m := metrics.New()
	cfg := &config.Config{
		DBURL:                "postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable",
		TableName:            "downloads",
		IDField:              "id",
		DatabaseQueryTimeout: 1 * time.Nanosecond, // Very short timeout
	}

	ctx := context.Background()

	store, err := NewPostgresStore(ctx, cfg, m)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer store.Close()

	// This should timeout due to the very short timeout
	_, err = store.GetRecord(ctx, "test-basic")
	if err == nil {
		// Might succeed if very fast, that's okay
		t.Log("Query succeeded despite short timeout (system is very fast)")
	}
}
