package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"

	"zipperfly/internal/config"
	"zipperfly/internal/handlers"
	"zipperfly/internal/metrics"
)

// newTestServer is a small helper to construct a Server with minimal deps.
func newTestServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()

	logger := zap.NewNop()
	m := metrics.New()

	// Zero-value handlers are fine here because we never actually invoke
	// their methods in these tests — we just need non-nil pointers for New().
	downloadHandler := &handlers.Handler{}
	healthHandler := &handlers.HealthHandler{}

	return New(logger, cfg, m, downloadHandler, healthHandler)
}

func TestNew_MetricsWithoutAuth(t *testing.T) {
	cfg := &config.Config{
		Port: "0", // ephemeral port (used only when Start() is called)
	}

	s := newTestServer(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /metrics without auth, got %d", w.Code)
	}

	if w.Body.Len() == 0 {
		t.Fatalf("expected non-empty metrics body")
	}
}

func TestNew_MetricsWithAuth(t *testing.T) {
	cfg := &config.Config{
		Port:            "0",
		MetricsUsername: "testuser",
		MetricsPassword: "testpass",
	}

	s := newTestServer(t, cfg)

	// 1) Without credentials → should NOT be 200
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	wNoAuth := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(wNoAuth, reqNoAuth)

	if wNoAuth.Code == http.StatusOK {
		t.Fatalf("expected non-200 for /metrics without auth, got %d", wNoAuth.Code)
	}

	// 2) With correct Basic Auth → should be 200
	reqAuth := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	reqAuth.SetBasicAuth("testuser", "testpass")
	wAuth := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(wAuth, reqAuth)

	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /metrics with valid auth, got %d", wAuth.Code)
	}

	if wAuth.Body.Len() == 0 {
		t.Fatalf("expected non-empty metrics body with auth")
	}
}

func TestServer_StartHTTPAndShutdown(t *testing.T) {
	cfg := &config.Config{
		Port: "0", // let the OS choose a free port
	}

	s := newTestServer(t, cfg)

	// Start() should kick off ListenAndServe in a goroutine and return nil.
	if err := s.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Give the server a brief moment to start listening.
	time.Sleep(50 * time.Millisecond)

	// Now gracefully shut down the server.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		t.Fatalf("Shutdown() returned error: %v", err)
	}
}

func TestServer_WaitForShutdown(t *testing.T) {
	cfg := &config.Config{
		Port: "0",
	}

	s := newTestServer(t, cfg)

	// Start the HTTP server so Shutdown() in WaitForShutdown has something to stop.
	if err := s.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	done := make(chan error, 1)

	// Run WaitForShutdown in the background; it should block until we send a signal.
	go func() {
		done <- s.WaitForShutdown()
	}()

	// Give WaitForShutdown time to register its signal.Notify.
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM to our own process; WaitForShutdown should catch this and
	// perform a graceful shutdown rather than killing the test process.
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess failed: %v", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("sending SIGTERM failed: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForShutdown returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown did not return within timeout")
	}
}
