package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		reqID := GetRequestID(r.Context())
		if reqID == "" {
			t.Error("request ID not found in context")
		}

		// Verify it's a valid UUID
		if _, err := uuid.Parse(reqID); err != nil {
			t.Errorf("request ID is not a valid UUID: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware(handler)

	t.Run("generates new request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		// Check header was set
		reqID := w.Header().Get("X-Request-ID")
		if reqID == "" {
			t.Error("X-Request-ID header not set in response")
		}

		// Verify it's a valid UUID
		if _, err := uuid.Parse(reqID); err != nil {
			t.Errorf("X-Request-ID is not a valid UUID: %v", err)
		}
	})

	t.Run("honors existing request ID", func(t *testing.T) {
		existingID := "550e8400-e29b-41d4-a716-446655440000"
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		// Check the existing ID was used
		reqID := w.Header().Get("X-Request-ID")
		if reqID != existingID {
			t.Errorf("X-Request-ID = %s, want %s", reqID, existingID)
		}
	})

	t.Run("different requests get different IDs", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/test", nil)
		w1 := httptest.NewRecorder()
		middleware.ServeHTTP(w1, req1)

		req2 := httptest.NewRequest("GET", "/test", nil)
		w2 := httptest.NewRecorder()
		middleware.ServeHTTP(w2, req2)

		id1 := w1.Header().Get("X-Request-ID")
		id2 := w2.Header().Get("X-Request-ID")

		if id1 == id2 {
			t.Errorf("expected different request IDs, got same: %s", id1)
		}
	})
}

func TestGetRequestID(t *testing.T) {
	t.Run("returns empty string for context without request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		reqID := GetRequestID(req.Context())

		if reqID != "" {
			t.Errorf("GetRequestID() = %s, want empty string", reqID)
		}
	})
}
