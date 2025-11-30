package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

func TestRedisStore_GetRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis test in short mode")
	}

	m := metrics.New()
	cfg := &config.Config{
		DBURL:                "redis://localhost:6379/0",
		KeyPrefix:            "test:",
		DatabaseQueryTimeout: 5 * time.Second,
	}

	ctx := context.Background()

	store, err := NewRedisStore(ctx, cfg, m)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer store.Close()

	// Insert a test record
	testRecord := &models.DownloadRecord{
		ID:      "test-redis-1",
		Bucket:  "test-bucket",
		Objects: []string{"file1.txt", "file2.txt"},
		Name:    "test-download",
	}

	data, err := json.Marshal(testRecord)
	if err != nil {
		t.Fatalf("failed to marshal test record: %v", err)
	}

	// Use the store's client to insert data
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer client.Close()

	err = client.Set(ctx, cfg.KeyPrefix+testRecord.ID, data, 0).Err()
	if err != nil {
		t.Skipf("failed to set test data in Redis: %v", err)
	}

	// Clean up after test
	defer client.Del(ctx, cfg.KeyPrefix+testRecord.ID)

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "existing record",
			id:      "test-redis-1",
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

			if record.Bucket != testRecord.Bucket {
				t.Errorf("record.Bucket = %s, want %s", record.Bucket, testRecord.Bucket)
			}

			if len(record.Objects) != len(testRecord.Objects) {
				t.Errorf("len(record.Objects) = %d, want %d", len(record.Objects), len(testRecord.Objects))
			}
		})
	}
}
