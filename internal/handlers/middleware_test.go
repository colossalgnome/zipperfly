package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	// Create a simple handler that the middleware will wrap
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name           string
		username       string
		password       string
		providedUser   string
		providedPass   string
		setAuth        bool
		wantStatus     int
		wantBody       string
		wantAuthHeader bool
	}{
		{
			name:       "valid credentials",
			username:   "admin",
			password:   "secret",
			providedUser: "admin",
			providedPass: "secret",
			setAuth:    true,
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "invalid username",
			username:   "admin",
			password:   "secret",
			providedUser: "wrong",
			providedPass: "secret",
			setAuth:    true,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Unauthorized\n",
			wantAuthHeader: true,
		},
		{
			name:       "invalid password",
			username:   "admin",
			password:   "secret",
			providedUser: "admin",
			providedPass: "wrong",
			setAuth:    true,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Unauthorized\n",
			wantAuthHeader: true,
		},
		{
			name:       "no credentials provided",
			username:   "admin",
			password:   "secret",
			setAuth:    false,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Unauthorized\n",
			wantAuthHeader: true,
		},
		{
			name:       "empty username and password allowed if configured",
			username:   "",
			password:   "",
			providedUser: "",
			providedPass: "",
			setAuth:    true,
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the test handler with BasicAuth middleware
			authMiddleware := BasicAuth(tt.username, tt.password)
			wrappedHandler := authMiddleware(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.providedUser, tt.providedPass)
			}

			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// Check response body
			if w.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", w.Body.String(), tt.wantBody)
			}

			// Check WWW-Authenticate header for 401 responses
			if tt.wantAuthHeader {
				authHeader := w.Header().Get("WWW-Authenticate")
				if authHeader == "" {
					t.Error("expected WWW-Authenticate header for 401 response")
				}
				expectedHeader := `Basic realm="metrics"`
				if authHeader != expectedHeader {
					t.Errorf("WWW-Authenticate = %q, want %q", authHeader, expectedHeader)
				}
			}
		})
	}
}
