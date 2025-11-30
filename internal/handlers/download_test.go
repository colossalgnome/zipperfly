package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"zipperfly/internal/auth"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// Shared metrics instance to avoid duplicate Prometheus registration
var sharedMetrics = metrics.New()

// mockDownloadDB implements database.Store for testing downloads
type mockDownloadDB struct {
	records map[string]*models.DownloadRecord
}

func (m *mockDownloadDB) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	if record, ok := m.records[id]; ok {
		return record, nil
	}
	return nil, errors.New("record not found")
}

func (m *mockDownloadDB) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockDownloadDB) Close() error {
	return nil
}

// mockDownloadStorage implements storage.Provider for testing downloads
type mockDownloadStorage struct {
	files map[string]string // bucket:key -> content
}

func (m *mockDownloadStorage) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	mapKey := fmt.Sprintf("%s:%s", bucket, key)
	if content, ok := m.files[mapKey]; ok {
		return io.NopCloser(strings.NewReader(content)), nil
	}
	return nil, errors.New("file not found")
}

func (m *mockDownloadStorage) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockDownloadStorage) Type() string {
	return "mock"
}


func TestHandler_Download(t *testing.T) {
	logger := zap.NewNop()
	m := sharedMetrics

	tests := []struct {
		name            string
		id              string
		records         map[string]*models.DownloadRecord
		files           map[string]string
		enforceSigning  bool
		queryExpiry     string
		querySignature  string
		ignoreMissing   bool
		wantStatus      int
		wantFilesInZip  []string
		checkCallback   bool
	}{
		{
			name: "missing id",
			id:   "",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file.txt"},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid signature",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file.txt"},
				},
			},
			enforceSigning: true,
			querySignature: "invalid-signature",
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name: "expired signature",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file.txt"},
				},
			},
			enforceSigning: true,
			queryExpiry:    "1", // Unix timestamp in the past
			querySignature: "some-sig",
			wantStatus:     http.StatusGone,
		},
		{
			name: "record not found",
			id:   "nonexistent",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file.txt"},
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "successful single file download",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file.txt"},
					Name:    "my-download",
				},
			},
			files: map[string]string{
				"bucket:file.txt": "Hello, World!",
			},
			wantStatus:     http.StatusOK,
			wantFilesInZip: []string{"file.txt"},
		},
		{
			name: "successful multi-file download",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"file1.txt", "file2.txt", "file3.txt"},
					Name:    "multi-download",
				},
			},
			files: map[string]string{
				"bucket:file1.txt": "Content 1",
				"bucket:file2.txt": "Content 2",
				"bucket:file3.txt": "Content 3",
			},
			wantStatus:     http.StatusOK,
			wantFilesInZip: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name: "missing file with ignoreMissing=false",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"exists.txt", "missing.txt"},
				},
			},
			files: map[string]string{
				"bucket:exists.txt": "I exist",
			},
			ignoreMissing: false,
			wantStatus:    http.StatusOK, // Returns 200 but with partial content
		},
		{
			name: "missing file with ignoreMissing=true",
			id:   "test",
			records: map[string]*models.DownloadRecord{
				"test": {
					ID:      "test",
					Bucket:  "bucket",
					Objects: []string{"exists.txt", "missing.txt"},
				},
			},
			files: map[string]string{
				"bucket:exists.txt": "I exist",
			},
			ignoreMissing:  true,
			wantStatus:     http.StatusOK,
			wantFilesInZip: []string{"exists.txt"}, // Only existing file in ZIP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDownloadDB{records: tt.records}
			storage := &mockDownloadStorage{files: tt.files}
			verifier := auth.NewVerifier([]byte("test-secret"), tt.enforceSigning, m)

			h := NewHandler(
				logger,
				db,
				storage,
				verifier,
				m,
				false, // appendYMD
				false, // sanitizeNames
				tt.ignoreMissing,
				10, // maxConcurrent
				0,  // callbackMaxRetries
				0,  // callbackRetryDelay
			)

			// Create request
			var req *http.Request
			if tt.id == "" {
				req = httptest.NewRequest("GET", "/download", nil)
			} else {
				url := fmt.Sprintf("/download/%s", tt.id)
				if tt.queryExpiry != "" || tt.querySignature != "" {
					url += "?"
					if tt.queryExpiry != "" {
						url += fmt.Sprintf("expiry=%s&", tt.queryExpiry)
					}
					if tt.querySignature != "" {
						url += fmt.Sprintf("signature=%s", tt.querySignature)
					}
				}
				req = httptest.NewRequest("GET", url, nil)
				req = mux.SetURLVars(req, map[string]string{"id": tt.id})
			}

			w := httptest.NewRecorder()
			h.Download(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// For successful downloads, verify ZIP contents
			if tt.wantStatus == http.StatusOK && len(tt.wantFilesInZip) > 0 {
				zipData := w.Body.Bytes()
				zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
				if err != nil {
					t.Fatalf("failed to read ZIP: %v", err)
				}

				if len(zipReader.File) != len(tt.wantFilesInZip) {
					t.Errorf("ZIP contains %d files, want %d", len(zipReader.File), len(tt.wantFilesInZip))
				}

				// Verify each expected file is in the ZIP
				fileMap := make(map[string]bool)
				for _, f := range zipReader.File {
					fileMap[f.Name] = true
				}

				for _, wantFile := range tt.wantFilesInZip {
					if !fileMap[wantFile] {
						t.Errorf("ZIP missing expected file: %s", wantFile)
					}
				}

				// Verify content-type and content-disposition headers
				if contentType := w.Header().Get("Content-Type"); contentType != "application/zip" {
					t.Errorf("Content-Type = %s, want application/zip", contentType)
				}

				contentDisp := w.Header().Get("Content-Disposition")
				if !strings.Contains(contentDisp, "attachment") {
					t.Errorf("Content-Disposition missing 'attachment': %s", contentDisp)
				}
			}
		})
	}
}

func TestHandler_PrepareFilename(t *testing.T) {
	tests := []struct {
		name          string
		inputName     string
		appendYMD     bool
		sanitizeNames bool
		wantContains  []string // Strings that should be in the result
		wantSuffix    string
	}{
		{
			name:         "empty name defaults to download",
			inputName:    "",
			wantContains: []string{"download"},
			wantSuffix:   ".zip",
		},
		{
			name:         "simple name",
			inputName:    "my-file",
			wantContains: []string{"my-file"},
			wantSuffix:   ".zip",
		},
		{
			name:         "strips .zip suffix",
			inputName:    "my-file.zip",
			wantContains: []string{"my-file"},
			wantSuffix:   ".zip",
		},
		{
			name:         "strips .ZIP suffix case insensitive",
			inputName:    "my-file.ZIP",
			wantContains: []string{"my-file"},
			wantSuffix:   ".zip",
		},
		{
			name:         "append YMD",
			inputName:    "report",
			appendYMD:    true,
			wantContains: []string{"report", "-"}, // Will have date suffix
			wantSuffix:   ".zip",
		},
		{
			name:          "sanitize invalid characters",
			inputName:     "file:with*invalid?chars",
			sanitizeNames: true,
			wantContains:  []string{"file_with_invalid_chars"},
			wantSuffix:    ".zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				zap.NewNop(),
				nil,
				nil,
				nil,
				sharedMetrics,
				tt.appendYMD,
				tt.sanitizeNames,
				false,
				10,
				0,
				0,
			)

			result := h.prepareFilename(tt.inputName)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result %q should contain %q", result, want)
				}
			}

			if !strings.HasSuffix(result, tt.wantSuffix) {
				t.Errorf("result %q should end with %q", result, tt.wantSuffix)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no change needed",
			input: "valid-filename_123",
			want:  "valid-filename_123",
		},
		{
			name:  "replace invalid chars",
			input: `file/name:with*invalid?"chars<>|`,
			want:  "file_name_with_invalid__chars___",
		},
		{
			name:  "control characters",
			input: "file\x00\x01\x1fname",
			want:  "file___name",
		},
		{
			name:  "trim spaces and dots",
			input: "  filename.txt  ",
			want:  "filename.txt",
		},
		{
			name:  "trim leading dots",
			input: "...filename",
			want:  "filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandler_SendCallback(t *testing.T) {
	tests := []struct {
		name       string
		serverCode int
		wantErr    bool
	}{
		{
			name:       "successful callback",
			serverCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server returns 2xx",
			serverCode: http.StatusCreated,
			wantErr:    false,
		},
		{
			name:       "server returns 4xx",
			serverCode: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name:       "server returns 5xx",
			serverCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != "POST" {
					t.Errorf("method = %s, want POST", r.Method)
				}

				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %s, want application/json", ct)
				}

				// Decode and verify payload
				var payload models.CallbackPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode payload: %v", err)
				}

				w.WriteHeader(tt.serverCode)
			}))
			defer server.Close()

			h := NewHandler(
				zap.NewNop(),
				nil,
				nil,
				nil,
				sharedMetrics,
				false,
				false,
				false,
				10,
				0,
				0,
			)

			payload := models.CallbackPayload{
				ID:         "test-id",
				Status:     "completed",
				Timestamp:  time.Now().Format(time.RFC3339),
				DurationMs: 1234,
			}

			err := h.sendCallback(server.URL, payload)

			if (err != nil) != tt.wantErr {
				t.Errorf("sendCallback() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_SendCallbackWithRetry(t *testing.T) {
	tests := []struct {
		name            string
		serverBehavior  []int // Response codes for each attempt
		maxRetries      int
		retryDelay      time.Duration
		wantAttempts    int
		wantMetricLabel string
	}{
		{
			name:            "success on first attempt",
			serverBehavior:  []int{http.StatusOK},
			maxRetries:      3,
			retryDelay:      1 * time.Millisecond,
			wantAttempts:    1,
			wantMetricLabel: "success",
		},
		{
			name:            "success on second attempt",
			serverBehavior:  []int{http.StatusInternalServerError, http.StatusOK},
			maxRetries:      3,
			retryDelay:      1 * time.Millisecond,
			wantAttempts:    2,
			wantMetricLabel: "success",
		},
		{
			name:            "all retries fail",
			serverBehavior:  []int{http.StatusInternalServerError, http.StatusInternalServerError, http.StatusInternalServerError, http.StatusInternalServerError},
			maxRetries:      3,
			retryDelay:      1 * time.Millisecond,
			wantAttempts:    4, // Initial + 3 retries
			wantMetricLabel: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attemptCount < len(tt.serverBehavior) {
					w.WriteHeader(tt.serverBehavior[attemptCount])
					attemptCount++
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					attemptCount++
				}
			}))
			defer server.Close()

			h := NewHandler(
				zap.NewNop(),
				nil,
				nil,
				nil,
				sharedMetrics,
				false,
				false,
				false,
				10,
				tt.maxRetries,
				tt.retryDelay,
			)

			payload := models.CallbackPayload{
				ID:         "test-id",
				Status:     "completed",
				Timestamp:  time.Now().Format(time.RFC3339),
				DurationMs: 1234,
			}

			// Run callback (it's async in real code, but we call it directly here)
			h.sendCallbackWithRetry(server.URL, payload)

			if attemptCount != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", attemptCount, tt.wantAttempts)
			}
		})
	}
}

func TestHandler_SendCallbackWithRetry_EmptyURL(t *testing.T) {
	h := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		sharedMetrics,
		false,
		false,
		false,
		10,
		3,
		1*time.Millisecond,
	)

	payload := models.CallbackPayload{
		ID:     "test-id",
		Status: "completed",
	}

	// Should return immediately without making any requests
	h.sendCallbackWithRetry("", payload)
	// If this doesn't panic or hang, the test passes
}
