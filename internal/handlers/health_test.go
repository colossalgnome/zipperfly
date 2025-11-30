package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"zipperfly/internal/models"
)

// Mock database store
type mockDB struct {
	shouldFail bool
}

func (m *mockDB) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	if m.shouldFail {
		return nil, context.DeadlineExceeded
	}
	if id == "__health_check__" {
		return nil, nil // Not found, but connection works
	}
	return &models.DownloadRecord{ID: id}, nil
}

func (m *mockDB) Close() error {
	return nil
}

// Mock storage provider
type mockStorage struct {
	shouldFail bool
}

func (m *mockStorage) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if m.shouldFail {
		return nil, context.DeadlineExceeded
	}
	return io.NopCloser(strings.NewReader("mock data")), nil
}

func (m *mockStorage) HealthCheck(ctx context.Context) error {
	if m.shouldFail {
		return context.DeadlineExceeded
	}
	return nil
}

func TestHealthHandler_Health(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	m := sharedMetrics

	tests := []struct {
		name               string
		dbFails            bool
		storageFails       bool
		wantStatus         int
		wantHealthy        bool
		wantDBStatus       string
		wantStorageStatus  string
	}{
		{
			name:              "all healthy",
			dbFails:           false,
			storageFails:      false,
			wantStatus:        http.StatusOK,
			wantHealthy:       true,
			wantDBStatus:      "ok",
			wantStorageStatus: "ok",
		},
		{
			name:              "database unhealthy",
			dbFails:           true,
			storageFails:      false,
			wantStatus:        http.StatusServiceUnavailable,
			wantHealthy:       false,
			wantDBStatus:      "unavailable",
			wantStorageStatus: "ok",
		},
		{
			name:              "storage unhealthy",
			dbFails:           false,
			storageFails:      true,
			wantStatus:        http.StatusServiceUnavailable,
			wantHealthy:       false,
			wantDBStatus:      "ok",
			wantStorageStatus: "unavailable",
		},
		{
			name:              "both unhealthy",
			dbFails:           true,
			storageFails:      true,
			wantStatus:        http.StatusServiceUnavailable,
			wantHealthy:       false,
			wantDBStatus:      "unavailable",
			wantStorageStatus: "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDB{shouldFail: tt.dbFails}
			storage := &mockStorage{shouldFail: tt.storageFails}

			handler := NewHealthHandler(logger, db, storage, m)

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			handler.Health(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Health() status = %d, want %d", w.Code, tt.wantStatus)
			}

			var resp healthResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			expectedStatus := "healthy"
			if !tt.wantHealthy {
				expectedStatus = "unhealthy"
			}

			if resp.Status != expectedStatus {
				t.Errorf("Health() status = %s, want %s", resp.Status, expectedStatus)
			}

			if resp.Checks["database"] != tt.wantDBStatus {
				t.Errorf("Health() database check = %s, want %s", resp.Checks["database"], tt.wantDBStatus)
			}

			if resp.Checks["storage"] != tt.wantStorageStatus {
				t.Errorf("Health() storage check = %s, want %s", resp.Checks["storage"], tt.wantStorageStatus)
			}

			if resp.Version == "" {
				t.Error("Health() version should not be empty")
			}
		})
	}
}
