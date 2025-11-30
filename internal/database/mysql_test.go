package database

import (
	"context"
	"testing"
	"time"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

func TestMySQLStore_GetRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mysql test in short mode")
	}

	m := metrics.New()
	cfg := &config.Config{
		DBURL:                "mysql://zipperfly:testpass@tcp(localhost:3306)/zipperfly_test",
		TableName:            "downloads",
		IDField:              "id",
		DatabaseQueryTimeout: 5 * time.Second,
	}

	store, err := NewMySQLStore(cfg, m)
	if err != nil {
		t.Skipf("MySQL not available: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

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

func TestMySQLStore_URLtoDSN(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "full mysql URL",
			url:  "mysql://user:pass@localhost:3306/dbname",
			want: "user:pass@tcp(localhost:3306)/dbname",
		},
		{
			name: "URL with query params",
			url:  "mysql://user:pass@localhost:3306/dbname?parseTime=true",
			want: "user:pass@tcp(localhost:3306)/dbname?parseTime=true",
		},
		{
			name: "already DSN format",
			url:  "user:pass@tcp(localhost:3306)/dbname",
			want: "user:pass@tcp(localhost:3306)/dbname",
		},
		{
			name: "URL without port",
			url:  "mysql://user:pass@localhost/dbname",
			want: "user:pass@tcp(localhost:3306)/dbname",
		},
		{
			name: "URL without password",
			url:  "mysql://user@localhost:3306/dbname",
			want: "user@tcp(localhost:3306)/dbname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mysqlURLtoDSN(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("mysqlURLtoDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("mysqlURLtoDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}
