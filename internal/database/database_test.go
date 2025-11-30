package database

import (
	"context"
	"strings"
	"testing"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
)

// fakeStore is a minimal in-memory implementation of Store for testing New.
// It never hits a real database.
type fakeStore struct {
	name string
}

func (f *fakeStore) GetRecord(ctx context.Context, id string) (*models.DownloadRecord, error) {
	return nil, nil
}

func (f *fakeStore) Close() error {
	return nil
}

func newTestConfig(engine string) *config.Config {
	return &config.Config{
		DBEngine: engine,
	}
}

func TestNew_PostgresDispatch(t *testing.T) {
	ctx := context.Background()
	m := metrics.New()

	cfg := newTestConfig("postgres")

	// Save and restore original function to avoid affecting other tests.
	orig := newPostgresStoreFunc
	defer func() { newPostgresStoreFunc = orig }()

	called := false
	expected := &fakeStore{name: "postgres"}

	newPostgresStoreFunc = func(c context.Context, cfg *config.Config, m *metrics.Metrics) (Store, error) {
		called = true
		return expected, nil
	}

	store, err := New(ctx, cfg, m)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !called {
		t.Fatalf("expected newPostgresStoreFunc to be called")
	}

	if store != expected {
		t.Fatalf("expected store %v, got %v", expected, store)
	}
}

func TestNew_MySQLDispatch(t *testing.T) {
	ctx := context.Background()
	m := metrics.New()

	cfg := newTestConfig("mysql")

	orig := newMySQLStoreFunc
	defer func() { newMySQLStoreFunc = orig }()

	called := false
	expected := &fakeStore{name: "mysql"}

	newMySQLStoreFunc = func(cfg *config.Config, m *metrics.Metrics) (Store, error) {
		called = true
		return expected, nil
	}

	store, err := New(ctx, cfg, m)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !called {
		t.Fatalf("expected newMySQLStoreFunc to be called")
	}

	if store != expected {
		t.Fatalf("expected store %v, got %v", expected, store)
	}
}

func TestNew_RedisDispatch(t *testing.T) {
	ctx := context.Background()
	m := metrics.New()

	cfg := newTestConfig("redis")

	orig := newRedisStoreFunc
	defer func() { newRedisStoreFunc = orig }()

	called := false
	expected := &fakeStore{name: "redis"}

	newRedisStoreFunc = func(c context.Context, cfg *config.Config, m *metrics.Metrics) (Store, error) {
		called = true
		return expected, nil
	}

	store, err := New(ctx, cfg, m)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !called {
		t.Fatalf("expected newRedisStoreFunc to be called")
	}

	if store != expected {
		t.Fatalf("expected store %v, got %v", expected, store)
	}
}

func TestNew_UnsupportedEngine(t *testing.T) {
	ctx := context.Background()
	m := metrics.New()

	cfg := newTestConfig("sqlite")

	store, err := New(ctx, cfg, m)
	if err == nil {
		t.Fatalf("expected error for unsupported engine, got nil")
	}

	if store != nil {
		t.Fatalf("expected nil store for unsupported engine, got %#v", store)
	}

	if !strings.Contains(err.Error(), "unsupported database engine") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
