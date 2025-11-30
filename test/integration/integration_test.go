//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
	"github.com/gorilla/mux"
	redislib "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"zipperfly/internal/auth"
	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/config"
	"zipperfly/internal/database"
	"zipperfly/internal/handlers"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
	"zipperfly/internal/storage"
)

const (
	fixturesDir = "../fixtures/files"
)

// One shared metrics instance to avoid duplicate Prometheus registrations.
var testMetrics = metrics.New()

var (
	s3SeedOnce sync.Once
	s3SeedErr  error
)

func ensureS3Fixtures(t *testing.T, bucket string) {
	t.Helper()

	s3SeedOnce.Do(func() {
		s3SeedErr = seedS3Fixtures(context.Background(), bucket)
	})

	if s3SeedErr != nil {
		t.Fatalf("failed to seed S3 fixtures: %v", s3SeedErr)
	}
}

func seedS3Fixtures(ctx context.Context, bucket string) error {
	// Hard-code the MinIO test endpoint & creds used by docker-compose
	const (
		endpoint  = "http://localhost:9000"
		region    = "us-east-1"
		accessKey = "minioadmin"
		secretKey = "minioadmin"
	)

	cfg, err := awscfg.LoadDefaultConfig(
		ctx,
		awscfg.WithRegion(region),
		awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		awscfg.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
					if service == s3.ServiceID {
						return aws.Endpoint{
							URL:              endpoint,
							HostnameImmutable: true, // path-style for MinIO
						}, nil
					}
					return aws.Endpoint{}, &aws.EndpointNotFoundError{}
				},
			),
		),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// 1) Ensure the bucket exists
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode()
			if code != "BucketAlreadyOwnedByYou" && code != "BucketAlreadyExists" {
				return err
			}
		} else {
			return err
		}
	}

	// 2) Upload all fixture files
	root := filepath.Join(getAbsPath(fixturesDir), bucket)

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(filepath.ToSlash(rel)),
			Body:   f,
		})
		return err
	})
}

// Helper to get absolute path for local storage fixtures
func getAbsPath(relPath string) string {
	abs, err := filepath.Abs(relPath)
	if err != nil {
		// Fallback to current directory + relpath
		wd, _ := os.Getwd()
		return filepath.Join(wd, relPath)
	}
	return abs
}

// Shared HTTP download test suite: runs against whatever backend the handler uses.
func runDownloadTests(t *testing.T, downloadHandler *handlers.Handler) {
	t.Helper()

	tests := []struct {
		name          string
		downloadID    string
		wantStatus    int
		wantFiles     []string
		checkZipValid bool
	}{
		{
			name:          "basic single file download",
			downloadID:    "test-basic",
			wantStatus:    http.StatusOK,
			wantFiles:     []string{"document.txt"},
			checkZipValid: true,
		},
		{
			name:          "multi-file download",
			downloadID:    "test-multi",
			wantStatus:    http.StatusOK,
			wantFiles:     []string{"document.txt", "data.json", "data.csv"},
			checkZipValid: true,
		},
		{
			name:          "all files download",
			downloadID:    "test-all",
			wantStatus:    http.StatusOK,
			wantFiles:     []string{"document.txt", "data.json", "data.csv", "binary.dat"},
			checkZipValid: true,
		},
		{
			name:          "download with missing file (ignore missing)",
			downloadID:    "test-missing",
			wantStatus:    http.StatusOK,
			wantFiles:     []string{"document.txt"}, // nonexistent.txt should be ignored
			checkZipValid: true,
		},
		{
			name:       "nonexistent download",
			downloadID: "nonexistent-id",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/download/" + tt.downloadID
			req := httptest.NewRequest("GET", url, nil)
			req = mux.SetURLVars(req, map[string]string{"id": tt.downloadID})

			w := httptest.NewRecorder()
			downloadHandler.Download(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
				t.Logf("response body: %s", w.Body.String())
				return
			}

			if tt.checkZipValid && w.Code == http.StatusOK {
				// Verify it's a valid ZIP
				zipData := w.Body.Bytes()
				zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
				if err != nil {
					t.Fatalf("invalid ZIP: %v", err)
				}

				// Check file count
				if len(zipReader.File) != len(tt.wantFiles) {
					t.Errorf("ZIP contains %d files, want %d", len(zipReader.File), len(tt.wantFiles))
				}

				// Check file names
				fileMap := make(map[string]bool)
				for _, f := range zipReader.File {
					fileMap[f.Name] = true
				}

				for _, wantFile := range tt.wantFiles {
					if !fileMap[wantFile] {
						t.Errorf("ZIP missing file: %s", wantFile)
					}
				}

				// Verify at least one file has content
				if len(zipReader.File) > 0 {
					rc, err := zipReader.File[0].Open()
					if err != nil {
						t.Fatalf("failed to open file in ZIP: %v", err)
					}
					defer rc.Close()

					content, err := io.ReadAll(rc)
					if err != nil {
						t.Fatalf("failed to read file content: %v", err)
					}

					if len(content) == 0 {
						t.Error("file in ZIP has zero content")
					}
				}
			}
		})
	}
}

// Common setup logic: given a config + optional seed function, build the handler and run HTTP tests.
func runDownloadSuite(t *testing.T, cfg *config.Config, seed func(ctx context.Context, t *testing.T)) {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	m := testMetrics
	ctx := context.Background()

	if seed != nil {
		seed(ctx, t)
	}

	// Create database connection
	db, err := database.New(ctx, cfg, m)
	if err != nil {
		t.Skipf("%s not available: %v (run: docker-compose -f docker-compose.test.yml up -d)", cfg.DBEngine, err)
	}
	defer db.Close()

	// Create storage provider (local filesystem or S3/MinIO depending on cfg)
	storageBreaker := circuitbreaker.New("storage", cfg, m)
	storageProvider, err := storage.New(ctx, cfg, m, storageBreaker)
	if err != nil {
		t.Fatalf("failed to create storage provider: %v", err)
	}

	// Create verifier and handler
	verifier := auth.NewVerifier(cfg.SigningSecret, cfg.EnforceSigning, m)
	downloadHandler := handlers.NewHandler(
		logger,
		db,
		storageProvider,
		verifier,
		m,
		false,               // no Prometheus handler wrapping here
		false,               // no debug logging injection
		cfg.IgnoreMissing,   // ignore missing files?
		cfg.MaxConcurrent,   // max concurrent file fetches
		cfg.CallbackMaxRetries,
		cfg.CallbackRetryDelay,
	)

	runDownloadTests(t, downloadHandler)
}

// =========================
//  PostgreSQL backend (local storage)
// =========================

func TestIntegration_LocalStorage_PostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.Config{
		DBURL:                     "postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable",
		DBEngine:                  "postgres",
		TableName:                 "downloads",
		IDField:                   "id",
		StorageType:               "local",
		StoragePath:               getAbsPath(fixturesDir),
		EnforceSigning:            false,
		SigningSecret:             []byte("test-secret"),
		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	// Postgres is already seeded via test/fixtures/postgres_schema.sql in your setup script
	runDownloadSuite(t, cfg, nil)
}

// =========================
//  MySQL backend (local storage)
// =========================

func TestIntegration_LocalStorage_MySQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.Config{
		DBURL:                     "mysql://zipperfly:testpass@localhost:3306/zipperfly_test",
		DBEngine:                  "mysql",
		TableName:                 "downloads",
		IDField:                   "id",
		StorageType:               "local",
		StoragePath:               getAbsPath(fixturesDir),
		EnforceSigning:            false,
		SigningSecret:             []byte("test-secret"),
		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	// MySQL is already seeded via test/fixtures/mysql_schema.sql in your setup script
	runDownloadSuite(t, cfg, nil)
}

// =========================
//  Redis backend (local storage)
// =========================

// Seed Redis with the same records as the SQL fixtures, using the RedisStore's expected format.
func seedRedisRecords(ctx context.Context, t *testing.T) {
	t.Helper()

	client := redislib.NewClient(&redislib.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer client.Close()

	testRecords := []models.DownloadRecord{
		{
			ID:      "test-basic",
			Bucket:  "test-bucket",
			Objects: []string{"document.txt"},
			Name:    "basic-download",
		},
		{
			ID:      "test-multi",
			Bucket:  "test-bucket",
			Objects: []string{"document.txt", "data.json", "data.csv"},
			Name:    "multi-file-download",
		},
		{
			ID:      "test-all",
			Bucket:  "test-bucket",
			Objects: []string{"document.txt", "data.json", "data.csv", "binary.dat"},
			Name:    "all-files",
		},
		{
			ID:      "test-missing",
			Bucket:  "test-bucket",
			Objects: []string{"document.txt", "nonexistent.txt"},
			Name:    "with-missing",
		},
	}

	for _, rec := range testRecords {
		data, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("failed to marshal redis test record %s: %v", rec.ID, err)
		}

		key := "dl:" + rec.ID // matches KeyPrefix "dl:"
		if err := client.Set(ctx, key, data, 0).Err(); err != nil {
			t.Fatalf("failed to seed redis key %s: %v", key, err)
		}
	}
}

func TestIntegration_LocalStorage_Redis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.Config{
		DBURL:                     "redis://localhost:6379/0",
		DBEngine:                  "redis",
		KeyPrefix:                 "dl:",
		TableName:                 "", // unused for Redis
		IDField:                   "", // unused for Redis
		StorageType:               "local",
		StoragePath:               getAbsPath(fixturesDir),
		EnforceSigning:            false,
		SigningSecret:             []byte("test-secret"),
		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	runDownloadSuite(t, cfg, seedRedisRecords)
}

// =========================
//  S3 / MinIO backends
// =========================

func TestIntegration_S3_PostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ensureS3Fixtures(t, "test-bucket")

	cfg := &config.Config{
		// DB
		DBURL:    "postgres://zipperfly:testpass@localhost:5432/zipperfly_test?sslmode=disable",
		DBEngine: "postgres",
		TableName: "downloads",
		IDField:   "id",

		// S3 / MinIO
		StorageType:       "s3",
		S3Endpoint:        "http://localhost:9000",
		S3Region:          "us-east-1",
		S3AccessKeyID:     "minioadmin",
		S3SecretAccessKey: "minioadmin",
		S3UsePathStyle: true,

		// Download behavior
		EnforceSigning: false,
		SigningSecret:  []byte("test-secret"),

		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	// DB rows already seeded by postgres_schema.sql; bucket/keys same as local tests.
	runDownloadSuite(t, cfg, nil)
}

func TestIntegration_S3_MySQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ensureS3Fixtures(t, "test-bucket")

	cfg := &config.Config{
		// DB
		DBURL:    "mysql://zipperfly:testpass@localhost:3306/zipperfly_test",
		DBEngine: "mysql",
		TableName: "downloads",
		IDField:   "id",

		// S3 / MinIO
		StorageType:       "s3",
		S3Endpoint:        "http://localhost:9000",
		S3Region:          "us-east-1",
		S3AccessKeyID:     "minioadmin",
		S3SecretAccessKey: "minioadmin",

		// Download behavior
		EnforceSigning: false,
		SigningSecret:  []byte("test-secret"),

		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	runDownloadSuite(t, cfg, nil)
}

func TestIntegration_S3_Redis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ensureS3Fixtures(t, "test-bucket")

	cfg := &config.Config{
		// DB (Redis)
		DBURL:    "redis://localhost:6379/0",
		DBEngine: "redis",
		KeyPrefix: "dl:",

		// S3 / MinIO
		StorageType:       "s3",
		S3Endpoint:        "http://localhost:9000",
		S3Region:          "us-east-1",
		S3AccessKeyID:     "minioadmin",
		S3SecretAccessKey: "minioadmin",

		// Download behavior
		EnforceSigning: false,
		SigningSecret:  []byte("test-secret"),

		DatabaseQueryTimeout:      5 * time.Second,
		StorageFetchTimeout:       10 * time.Second,
		RequestTimeout:            30 * time.Second,
		MaxFileSize:               0,
		MaxFilesPerRequest:        0,
		StorageMaxRetries:         3,
		StorageRetryDelay:         time.Second,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     10 * time.Second,
		CircuitBreakerMaxRequests: 2,
		MaxConcurrent:             10,
		IgnoreMissing:             true,
		CallbackMaxRetries:        3,
		CallbackRetryDelay:        time.Second,
	}

	runDownloadSuite(t, cfg, seedRedisRecords)
}
