package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"zipperfly/internal/metrics"
)

// Verifier handles request signature verification
type Verifier struct {
	secret         []byte
	enforceSigning bool
	metrics        *metrics.Metrics
}

// NewVerifier creates a new signature verifier
func NewVerifier(secret []byte, enforceSigning bool, m *metrics.Metrics) *Verifier {
	return &Verifier{
		secret:         secret,
		enforceSigning: enforceSigning,
		metrics:        m,
	}
}

// Verify checks the signature and expiry of a request
func (v *Verifier) Verify(id, expiryStr, signature string) error {
	hasExpiry := expiryStr != ""

	// Check expiry if provided
	if hasExpiry {
		expiry, err := strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid expiry: %w", err)
		}
		if time.Now().Unix() > expiry {
			v.metrics.ExpiredRequestsTotal.Inc()
			return fmt.Errorf("request has expired")
		}
	}

	// Check signature if enforced or provided
	if v.enforceSigning || signature != "" {
		if signature == "" {
			v.metrics.SignatureFailuresTotal.Inc()
			return fmt.Errorf("signature required")
		}

		payload := id
		if hasExpiry {
			payload += "|" + expiryStr
		}

		h := hmac.New(sha256.New, v.secret)
		h.Write([]byte(payload))
		expectedSig := hex.EncodeToString(h.Sum(nil))

		if signature != expectedSig {
			v.metrics.SignatureFailuresTotal.Inc()
			return fmt.Errorf("invalid signature")
		}
	}

	return nil
}
