package metrics

import (
    "runtime"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    defaultMetrics *Metrics
    metricsOnce    sync.Once
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP requests
	RequestsTotal *prometheus.CounterVec

	// Download outcomes
	DownloadsTotal *prometheus.CounterVec // by status: completed, failed, partial

	// File-level metrics
	FilesRequestedHist prometheus.Histogram // Total files requested per download
	FilesSuccessHist   prometheus.Histogram // Files successfully fetched per download
	FilesFetchTotal    *prometheus.CounterVec // Total file fetches by result: success, missing, error
	MissingFilesTotal  prometheus.Counter // Total count of missing files encountered

	// Performance metrics
	DurationHist      prometheus.Histogram
	OutgoingBytesHist prometheus.Histogram
	IncomingBytesHist prometheus.Histogram

	// Backend performance
	DatabaseQueryDuration *prometheus.HistogramVec // DB query latency by db_type
	StorageFetchDuration  *prometheus.HistogramVec // Storage fetch latency by storage_type

	// Authentication/Security
	SignatureFailuresTotal prometheus.Counter
	ExpiredRequestsTotal   prometheus.Counter

	// Callback metrics
	CallbacksTotal    *prometheus.CounterVec // by status: success, failure
	CallbackRetries   prometheus.Counter

	// Concurrency
	ActiveDownloads    prometheus.Gauge
	ActiveFileFetches  prometheus.Gauge

	// ZIP statistics
	CompressionRatio prometheus.Histogram

	// Client behavior
	ClientDisconnectsTotal prometheus.Counter

	// Circuit breaker
	CircuitBreakerState *prometheus.GaugeVec // by backend: storage, database

	// Health checks
	HealthStatus      *prometheus.GaugeVec // by component: database, storage (1=healthy, 0=unhealthy)
	HealthChecksFailed *prometheus.CounterVec // by component: database, storage

	// System metrics
	MemoryGauge     prometheus.Gauge
	GoroutinesGauge prometheus.Gauge
}

// New creates and registers all metrics
func New() *Metrics {
    metricsOnce.Do(func() {
        defaultMetrics = &Metrics{
            // HTTP requests
            RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
                Name: "zipperfly_requests_total",
                Help: "Total number of HTTP requests by status code",
            }, []string{"status"}),

            // Download outcomes
            DownloadsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
                Name: "zipperfly_downloads_total",
                Help: "Total number of download attempts by outcome (completed, failed, partial)",
            }, []string{"status"}),

            // File-level metrics
            FilesRequestedHist: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_files_requested",
                Help:    "Number of files requested per download",
                Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500, 1000, 5000},
            }),
            FilesSuccessHist: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_files_success",
                Help:    "Number of files successfully fetched per download",
                Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500, 1000, 5000},
            }),
            FilesFetchTotal: promauto.NewCounterVec(prometheus.CounterOpts{
                Name: "zipperfly_files_fetch_total",
                Help: "Total file fetch attempts by result (success, missing, error)",
            }, []string{"result"}),
            MissingFilesTotal: promauto.NewCounter(prometheus.CounterOpts{
                Name: "zipperfly_missing_files_total",
                Help: "Total count of missing files encountered across all downloads",
            }),

            // Performance metrics
            DurationHist: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_request_duration_seconds",
                Help:    "Request duration in seconds",
                Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800}, // 1s to 30min
            }),
            OutgoingBytesHist: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_outgoing_bytes",
                Help:    "Outgoing bytes per response (compressed ZIP size)",
                Buckets: prometheus.ExponentialBuckets(1024, 2, 35), // Up to ~32GB+
            }),
            IncomingBytesHist: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_incoming_bytes",
                Help:    "Incoming bytes from storage per request (uncompressed)",
                Buckets: prometheus.ExponentialBuckets(1024, 2, 35), // Up to ~32GB+
            }),

            // Backend performance
            DatabaseQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
                Name:    "zipperfly_database_query_duration_seconds",
                Help:    "Database query duration in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
            }, []string{"db_type"}),
            StorageFetchDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
                Name:    "zipperfly_storage_fetch_duration_seconds",
                Help:    "Storage fetch duration per file in seconds",
                Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
            }, []string{"storage_type", "result"}),

            // Authentication/Security
            SignatureFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
                Name: "zipperfly_signature_failures_total",
                Help: "Total number of failed signature verifications",
            }),
            ExpiredRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
                Name: "zipperfly_expired_requests_total",
                Help: "Total number of requests with expired timestamps",
            }),

            // Callback metrics
            CallbacksTotal: promauto.NewCounterVec(prometheus.CounterOpts{
                Name: "zipperfly_callbacks_total",
                Help: "Total number of callback attempts by status",
            }, []string{"status"}),
            CallbackRetries: promauto.NewCounter(prometheus.CounterOpts{
                Name: "zipperfly_callback_retries_total",
                Help: "Total number of callback retry attempts",
            }),

            // Concurrency
            ActiveDownloads: promauto.NewGauge(prometheus.GaugeOpts{
                Name: "zipperfly_active_downloads",
                Help: "Number of currently active downloads",
            }),
            ActiveFileFetches: promauto.NewGauge(prometheus.GaugeOpts{
                Name: "zipperfly_active_file_fetches",
                Help: "Number of currently active file fetches",
            }),

            // ZIP statistics
            CompressionRatio: promauto.NewHistogram(prometheus.HistogramOpts{
                Name:    "zipperfly_compression_ratio",
                Help:    "Compression ratio (compressed/uncompressed)",
                Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
            }),

            // Client behavior
            ClientDisconnectsTotal: promauto.NewCounter(prometheus.CounterOpts{
                Name: "zipperfly_client_disconnects_total",
                Help: "Total number of client disconnects during download",
            }),

            // Circuit breaker
            CircuitBreakerState: promauto.NewGaugeVec(prometheus.GaugeOpts{
                Name: "zipperfly_circuit_breaker_state",
                Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
            }, []string{"backend"}),

            // Health checks
            HealthStatus: promauto.NewGaugeVec(prometheus.GaugeOpts{
                Name: "zipperfly_health_status",
                Help: "Health status by component (1=healthy, 0=unhealthy)",
            }, []string{"component"}),
            HealthChecksFailed: promauto.NewCounterVec(prometheus.CounterOpts{
                Name: "zipperfly_health_checks_failed_total",
                Help: "Total number of failed health checks by component",
            }, []string{"component"}),

            // System metrics
            MemoryGauge: promauto.NewGauge(prometheus.GaugeOpts{
                Name: "zipperfly_memory_heap_alloc_bytes",
                Help: "Current heap allocation in bytes",
            }),
            GoroutinesGauge: promauto.NewGauge(prometheus.GaugeOpts{
                Name: "zipperfly_goroutines",
                Help: "Number of goroutines",
            }),
	    }
    })

    return defaultMetrics
}

// StartRuntimeMetricsCollector starts a goroutine that updates runtime metrics
func (m *Metrics) StartRuntimeMetricsCollector() {
	go func() {
		for {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			m.MemoryGauge.Set(float64(mem.HeapAlloc))
			m.GoroutinesGauge.Set(float64(runtime.NumGoroutine()))
			time.Sleep(10 * time.Second)
		}
	}()
}
