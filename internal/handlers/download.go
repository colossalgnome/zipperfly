package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"zipperfly/internal/auth"
	"zipperfly/internal/database"
	"zipperfly/internal/metrics"
	"zipperfly/internal/models"
	"zipperfly/internal/storage"
)

// Handler handles download requests
type Handler struct {
	logger              *zap.Logger
	db                  database.Store
	storage             storage.Provider
	verifier            *auth.Verifier
	metrics             *metrics.Metrics
	appendYMD           bool
	sanitizeNames       bool
	ignoreMissing       bool
	maxConcurrent       int64
	callbackMaxRetries  int
	callbackRetryDelay  time.Duration
}

// NewHandler creates a new download handler
func NewHandler(
	logger *zap.Logger,
	db database.Store,
	storageProvider storage.Provider,
	verifier *auth.Verifier,
	m *metrics.Metrics,
	appendYMD bool,
	sanitizeNames bool,
	ignoreMissing bool,
	maxConcurrent int64,
	callbackMaxRetries int,
	callbackRetryDelay time.Duration,
) *Handler {
	return &Handler{
		logger:             logger,
		db:                 db,
		storage:            storageProvider,
		verifier:           verifier,
		metrics:            m,
		appendYMD:          appendYMD,
		sanitizeNames:      sanitizeNames,
		ignoreMissing:      ignoreMissing,
		maxConcurrent:      maxConcurrent,
		callbackMaxRetries: callbackMaxRetries,
		callbackRetryDelay: callbackRetryDelay,
	}
}

// Download handles the download request
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Track active downloads
	h.metrics.ActiveDownloads.Inc()
	defer h.metrics.ActiveDownloads.Dec()

	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		h.metrics.RequestsTotal.WithLabelValues("400").Inc()
		return
	}

	query := r.URL.Query()
	expiryStr := query.Get("expiry")
	sig := query.Get("signature")

	// Verify signature and expiry
	if err := h.verifier.Verify(id, expiryStr, sig); err != nil {
		statusCode := http.StatusUnauthorized
		if strings.Contains(err.Error(), "expired") {
			statusCode = http.StatusGone
			h.logger.Warn("expired request", zap.String("id", id))
		} else {
			h.logger.Warn("verification failed", zap.String("id", id), zap.Error(err))
		}
		http.Error(w, err.Error(), statusCode)
		h.metrics.RequestsTotal.WithLabelValues(fmt.Sprintf("%d", statusCode)).Inc()
		return
	}

	// Get record from database
	record, err := h.db.GetRecord(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		h.logger.Error("record not found", zap.Error(err), zap.String("id", id))
		h.metrics.RequestsTotal.WithLabelValues("404").Inc()
		return
	}

	// Prepare filename
	filename := h.prepareFilename(record.Name)

	// Set response headers
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Create ZIP writer with byte counting
	outBc := &models.ByteCounter{Writer: w}
	zw := zip.NewWriter(outBc)
	defer zw.Close()

	// Stream files from storage
	var inBytes int64
	successCount, fetchErr := h.streamFilesFromStorage(ctx, zw, record, &inBytes)

	// Check if client disconnected
	if ctx.Err() != nil {
		h.metrics.ClientDisconnectsTotal.Inc()
		h.logger.Warn("client disconnected", zap.String("id", id), zap.Error(ctx.Err()))
		// Still continue to finish the request and metrics
	}

	// Determine download status
	status := "completed"
	message := ""
	if fetchErr != nil {
		status = "failed"
		message = fetchErr.Error()
		h.logger.Error("fetch error", zap.Error(fetchErr), zap.String("id", id))
	} else if successCount < len(record.Objects) {
		// Some files were missing but we continued (ignoreMissing=true)
		status = "partial"
		message = fmt.Sprintf("processed %d of %d files (some files missing)", successCount, len(record.Objects))
		h.logger.Warn("incomplete download", zap.String("id", id), zap.Int("success", successCount), zap.Int("requested", len(record.Objects)))
	}

	// Record metrics
	duration := time.Since(start)

	// Performance metrics
	h.metrics.DurationHist.Observe(duration.Seconds())
	h.metrics.OutgoingBytesHist.Observe(float64(outBc.Count))
	h.metrics.IncomingBytesHist.Observe(float64(inBytes))

	// Compression ratio (compressed/uncompressed)
	if inBytes > 0 {
		ratio := float64(outBc.Count) / float64(inBytes)
		h.metrics.CompressionRatio.Observe(ratio)
	}

	// Download outcome metrics
	h.metrics.DownloadsTotal.WithLabelValues(status).Inc()
	h.metrics.RequestsTotal.WithLabelValues("200").Inc()

	// File-level metrics
	h.metrics.FilesRequestedHist.Observe(float64(len(record.Objects)))
	h.metrics.FilesSuccessHist.Observe(float64(successCount))

	// Send callback
	go h.sendCallbackWithRetry(record.Callback, models.CallbackPayload{
		ID:                  id,
		Status:              status,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
		Message:             message,
		DurationMs:          duration.Milliseconds(),
		FileCount:           len(record.Objects),
		CompressedSizeBytes: outBc.Count,
	})

	h.logger.Info("download handled", zap.String("id", id), zap.String("status", status), zap.Duration("duration", duration))
}

func (h *Handler) prepareFilename(name string) string {
	filename := name
	if filename == "" {
		filename = "download"
	} else if h.sanitizeNames {
		filename = sanitizeFilename(filename)
	}

	// Strip .zip if present
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		filename = filename[:len(filename)-4]
	}

	if h.appendYMD {
		filename += "-" + time.Now().Format("20060102")
	}

	filename += ".zip"
	return filename
}

func (h *Handler) streamFilesFromStorage(
    ctx context.Context,
    zw *zip.Writer,
    record *models.DownloadRecord,
    inBytes *int64,
) (int, error) {
    sem := semaphore.NewWeighted(h.maxConcurrent)
    var zipMu sync.Mutex

    type result struct {
        err     error
        success bool
    }
    resultChan := make(chan result, len(record.Objects))

    for _, obj := range record.Objects {
        key := obj

        go func(key string) {
            if err := sem.Acquire(ctx, 1); err != nil {
                h.metrics.FilesFetchTotal.WithLabelValues("error").Inc()
                resultChan <- result{err: err, success: false}
                return
            }
            defer sem.Release(1)

            // Get object from storage provider
            body, err := h.storage.GetObject(ctx, record.Bucket, key)
            if err != nil {
                if h.ignoreMissing {
                    h.logger.Warn(
                        "skipping missing file",
                        zap.String("bucket", record.Bucket),
                        zap.String("key", key),
                        zap.Error(err),
                    )
                    h.metrics.FilesFetchTotal.WithLabelValues("missing").Inc()
                    h.metrics.MissingFilesTotal.Inc()
                    resultChan <- result{err: nil, success: false}
                    return
                }

                h.metrics.FilesFetchTotal.WithLabelValues("error").Inc()
                resultChan <- result{err: err, success: false}
                return
            }
            defer body.Close()

            // --- Serialize ZIP writing ---
            zipMu.Lock()
            header := &zip.FileHeader{
                Name:   filepath.Base(key),
                Method: zip.Deflate,
            }

            fw, err := zw.CreateHeader(header)
            if err != nil {
                zipMu.Unlock()
                h.metrics.FilesFetchTotal.WithLabelValues("error").Inc()
                resultChan <- result{err: err, success: false}
                return
            }

            // Wrap writer to count bytes
            inBc := &models.ByteCounter{Writer: fw}

            // Copy data from body -> ZIP entry
            buf := make([]byte, 32*1024)
            for {
                n, readErr := body.Read(buf)
                if n > 0 {
                    if _, writeErr := inBc.Write(buf[:n]); writeErr != nil {
                        zipMu.Unlock()
                        h.metrics.FilesFetchTotal.WithLabelValues("error").Inc()
                        resultChan <- result{err: writeErr, success: false}
                        return
                    }
                }

                if readErr != nil {
                    if readErr == io.EOF {
                        break
                    }

                    zipMu.Unlock()
                    h.metrics.FilesFetchTotal.WithLabelValues("error").Inc()
                    resultChan <- result{err: readErr, success: false}
                    return
                }
            }

            zipMu.Unlock()
            // --- end critical section ---

            atomic.AddInt64(inBytes, inBc.Count)
            h.metrics.FilesFetchTotal.WithLabelValues("success").Inc()
            resultChan <- result{err: nil, success: true}
        }(key)
    }

    var fetchErr error
    successCount := 0

    for range record.Objects {
        res := <-resultChan
        if res.success {
            successCount++
        } else if res.err != nil && fetchErr == nil {
            // Store first error encountered
            fetchErr = res.err
        }
    }

    // If ignoring missing files, only fail if ALL files failed
    if h.ignoreMissing && successCount == 0 && len(record.Objects) > 0 {
        return 0, fmt.Errorf("all %d files missing or failed to fetch", len(record.Objects))
    }

    // If not ignoring missing and we had an error, return it
    if !h.ignoreMissing && fetchErr != nil {
        return successCount, fetchErr
    }

    return successCount, nil
}

// sendCallbackWithRetry sends a callback with exponential backoff retry logic
func (h *Handler) sendCallbackWithRetry(url string, payload models.CallbackPayload) {
	if url == "" {
		return
	}

	for attempt := 0; attempt <= h.callbackMaxRetries; attempt++ {
		if attempt > 0 {
			h.metrics.CallbackRetries.Inc()
			// Exponential backoff: callbackRetryDelay * 2^(attempt-1)
			delay := h.callbackRetryDelay * time.Duration(1<<(attempt-1))
			time.Sleep(delay)
			h.logger.Info("retrying callback", zap.String("url", url), zap.Int("attempt", attempt))
		}

		err := h.sendCallback(url, payload)
		if err == nil {
			h.metrics.CallbacksTotal.WithLabelValues("success").Inc()
			return
		}

		h.logger.Warn("callback attempt failed", zap.String("url", url), zap.Int("attempt", attempt), zap.Error(err))

		// If this was the last attempt, record failure
		if attempt == h.callbackMaxRetries {
			h.metrics.CallbacksTotal.WithLabelValues("failure").Inc()
			h.logger.Error("callback failed after retries", zap.String("url", url), zap.Int("total_attempts", attempt+1), zap.Error(err))
		}
	}
}

// sendCallback sends a single callback request
func (h *Handler) sendCallback(url string, payload models.CallbackPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Set a reasonable timeout for callback requests
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	return nil
}

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	return name
}
