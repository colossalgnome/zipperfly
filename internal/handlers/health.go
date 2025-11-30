package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"zipperfly/internal/database"
	"zipperfly/internal/metrics"
	"zipperfly/internal/storage"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	logger  *zap.Logger
	db      database.Store
	storage storage.Provider
	metrics *metrics.Metrics
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(logger *zap.Logger, db database.Store, storageProvider storage.Provider, m *metrics.Metrics) *HealthHandler {
	return &HealthHandler{
		logger:  logger,
		db:      db,
		storage: storageProvider,
		metrics: m,
	}
}

type healthResponse struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks,omitempty"`
	Version string            `json:"version,omitempty"`
}

// Health returns health status (checks dependencies)
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	allHealthy := true

	// Check database connectivity
	dbHealthy := h.checkDatabase(ctx)
	if dbHealthy {
		checks["database"] = "ok"
		h.metrics.HealthStatus.WithLabelValues("database").Set(1)
	} else {
		checks["database"] = "unavailable"
		allHealthy = false
		h.metrics.HealthStatus.WithLabelValues("database").Set(0)
		h.metrics.HealthChecksFailed.WithLabelValues("database").Inc()
		h.logger.Warn("database health check failed")
	}

	// Check storage connectivity
	storageHealthy := h.checkStorage(ctx)
	if storageHealthy {
		checks["storage"] = "ok"
		h.metrics.HealthStatus.WithLabelValues("storage").Set(1)
	} else {
		checks["storage"] = "unavailable"
		allHealthy = false
		h.metrics.HealthStatus.WithLabelValues("storage").Set(0)
		h.metrics.HealthChecksFailed.WithLabelValues("storage").Inc()
		h.logger.Warn("storage health check failed")
	}

	w.Header().Set("Content-Type", "application/json")
	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(healthResponse{
		Status:  map[bool]string{true: "healthy", false: "unhealthy"}[allHealthy],
		Checks:  checks,
		Version: "1.0.0",
	})
}

func (h *HealthHandler) checkDatabase(ctx context.Context) bool {
	// Try to perform a simple operation with timeout
	_, err := h.db.GetRecord(ctx, "__health_check__")
	// We expect this to fail (record doesn't exist), but if it fails due to
	// connection issues (timeout/unavailable), that's what we're checking for
	if err == nil {
		return true // Unexpectedly found the record, but DB is working
	}
	// Check if error is a timeout (bad) vs not found (good)
	errStr := err.Error()
	return errStr != "context deadline exceeded" && errStr != "connection refused"
}

func (h *HealthHandler) checkStorage(ctx context.Context) bool {
	// Use the storage provider's built-in health check
	err := h.storage.HealthCheck(ctx)
	return err == nil
}
