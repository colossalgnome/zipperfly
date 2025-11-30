package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"zipperfly/internal/metrics"
)

func TestVerifier_Verify(t *testing.T) {
	secret := []byte("test-secret")
	m := metrics.New()

	tests := []struct {
		name          string
		enforceSigning bool
		id            string
		expiryStr     string
		signature     string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid signature without expiry",
			enforceSigning: false,
			id:            "test-id",
			expiryStr:     "",
			signature:     generateSignature(secret, "test-id", ""),
			wantErr:       false,
		},
		{
			name:          "valid signature with future expiry",
			enforceSigning: false,
			id:            "test-id",
			expiryStr:     strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10),
			signature:     "", // will be generated in test
			wantErr:       false,
		},
		{
			name:          "expired request",
			enforceSigning: false,
			id:            "test-id",
			expiryStr:     strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10),
			signature:     "",
			wantErr:       true,
			errContains:   "expired",
		},
		{
			name:          "invalid expiry format",
			enforceSigning: false,
			id:            "test-id",
			expiryStr:     "not-a-number",
			signature:     "",
			wantErr:       true,
			errContains:   "invalid expiry",
		},
		{
			name:          "enforce signing without signature",
			enforceSigning: true,
			id:            "test-id",
			expiryStr:     "",
			signature:     "",
			wantErr:       true,
			errContains:   "signature required",
		},
		{
			name:          "invalid signature",
			enforceSigning: true,
			id:            "test-id",
			expiryStr:     "",
			signature:     "invalid-signature",
			wantErr:       true,
			errContains:   "invalid signature",
		},
		{
			name:          "no enforcement, no signature - allowed",
			enforceSigning: false,
			id:            "test-id",
			expiryStr:     "",
			signature:     "",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVerifier(secret, tt.enforceSigning, m)

			// Generate signature if needed and not testing invalid cases
			sig := tt.signature
			if sig == "" && !tt.wantErr && tt.name == "valid signature with future expiry" {
				sig = generateSignature(secret, tt.id, tt.expiryStr)
			}

			err := v.Verify(tt.id, tt.expiryStr, sig)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Verify() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Verify() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Verify() unexpected error = %v", err)
				}
			}
		})
	}
}

func generateSignature(secret []byte, id, expiryStr string) string {
	payload := id
	if expiryStr != "" {
		payload += "|" + expiryStr
	}
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}
