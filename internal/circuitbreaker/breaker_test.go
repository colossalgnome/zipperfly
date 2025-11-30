package circuitbreaker

import (
	"errors"
	"testing"
	"time"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

func TestCircuitBreaker(t *testing.T) {
	m := metrics.New()
	cfg := &config.Config{
		CircuitBreakerThreshold:   3, // Open after 3 failures
		CircuitBreakerTimeout:     100 * time.Millisecond,
		CircuitBreakerMaxRequests: 1,
	}

	cb := New("test", cfg, m)

	t.Run("successful requests keep circuit closed", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			_, err := cb.Execute(func() (interface{}, error) {
				return "success", nil
			})
			if err != nil {
				t.Errorf("Execute() error = %v, want nil", err)
			}
		}
	})

	t.Run("multiple failures open circuit", func(t *testing.T) {
		testErr := errors.New("test error")

		// Trigger failures to open circuit
		for i := 0; i < 4; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, testErr
			})
		}

		// Circuit should be open now, rejecting requests
		_, err := cb.Execute(func() (interface{}, error) {
			t.Error("function should not be called when circuit is open")
			return nil, nil
		})

		if err == nil {
			t.Error("Execute() should return error when circuit is open")
		}
	})

	t.Run("circuit recovers after timeout", func(t *testing.T) {
		// Wait for circuit to enter half-open state
		time.Sleep(150 * time.Millisecond)

		// Successful request should close circuit
		_, err := cb.Execute(func() (interface{}, error) {
			return "recovered", nil
		})

		if err != nil {
			t.Errorf("Execute() after timeout error = %v, want nil", err)
		}
	})
}
