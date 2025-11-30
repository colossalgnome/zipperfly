package circuitbreaker

import (
	"github.com/sony/gobreaker"

	"zipperfly/internal/config"
	"zipperfly/internal/metrics"
)

// Breaker wraps gobreaker with metrics
type Breaker struct {
	cb      *gobreaker.CircuitBreaker
	metrics *metrics.Metrics
	name    string
}

// New creates a new circuit breaker
func New(name string, cfg *config.Config, m *metrics.Metrics) *Breaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: uint32(cfg.CircuitBreakerMaxRequests),
		Interval:    cfg.CircuitBreakerTimeout,
		Timeout:     cfg.CircuitBreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(cfg.CircuitBreakerThreshold)
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// Update metrics
			m.CircuitBreakerState.WithLabelValues(name).Set(float64(to))
		},
	}

	return &Breaker{
		cb:      gobreaker.NewCircuitBreaker(settings),
		metrics: m,
		name:    name,
	}
}

// Execute runs the given function through the circuit breaker
func (b *Breaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return b.cb.Execute(fn)
}

// State returns the current state of the circuit breaker
func (b *Breaker) State() gobreaker.State {
	return b.cb.State()
}
