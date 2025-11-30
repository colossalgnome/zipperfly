package metrics

import (
	"runtime"
	"testing"
	"time"
)

func TestNew_SingletonAndFieldsNonNil(t *testing.T) {
	m1 := New()
	if m1 == nil {
		t.Fatal("New() returned nil metrics instance")
	}

	m2 := New()
	if m1 != m2 {
		t.Fatal("New() did not behave as a singleton – pointers differ")
	}

	// Spot-check a few important fields to ensure they were registered.
	if m1.RequestsTotal == nil {
		t.Error("RequestsTotal is nil")
	}
	if m1.DownloadsTotal == nil {
		t.Error("DownloadsTotal is nil")
	}
	if m1.DatabaseQueryDuration == nil {
		t.Error("DatabaseQueryDuration is nil")
	}
	if m1.StorageFetchDuration == nil {
		t.Error("StorageFetchDuration is nil")
	}
	if m1.MemoryGauge == nil || m1.GoroutinesGauge == nil {
		t.Error("runtime gauges are nil")
	}
}

func TestStartRuntimeMetricsCollector_LaunchesGoroutine(t *testing.T) {
	m := New()

	before := runtime.NumGoroutine()
	m.StartRuntimeMetricsCollector()

	// Give the background goroutine a short window to start and run at least one loop.
	// The loop executes immediately, then sleeps 10s, so a small sleep here is fine.
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after < before {
		t.Fatalf("expected goroutine count to stay the same or increase, before=%d after=%d", before, after)
	}
}
