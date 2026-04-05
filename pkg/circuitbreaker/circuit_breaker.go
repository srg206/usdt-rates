// Package circuitbreaker wraps github.com/sony/gobreaker with app-level settings.
package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// Settings configures a Sony gobreaker instance.
type Settings struct {
	// Name is the breaker name in telemetry (e.g. "grinex"). Empty defaults to "grinex".
	Name string
	// Enabled turns the breaker on. When false, New returns (nil, nil).
	Enabled bool
	// ConsecutiveFailures trips the breaker to open after this many failures in a row.
	ConsecutiveFailures uint32
	// OpenTimeout is how long the breaker stays open before moving to half-open.
	OpenTimeout time.Duration
	// HalfOpenMaxRequests is the max allowed calls in half-open state.
	HalfOpenMaxRequests uint32
	// Interval resets internal counts in the closed state on this period; 0 disables resets.
	Interval time.Duration
}

// New builds a *gobreaker.CircuitBreaker from Settings, or (nil, nil) if Enabled is false.
func New(log *zap.Logger, s Settings) (*gobreaker.CircuitBreaker, error) {
	if !s.Enabled {
		return nil, nil
	}
	if s.ConsecutiveFailures < 1 {
		return nil, fmt.Errorf("circuit breaker: consecutive failures threshold must be >= 1")
	}
	if s.OpenTimeout <= 0 {
		return nil, fmt.Errorf("circuit breaker: open timeout must be > 0")
	}
	if s.HalfOpenMaxRequests < 1 {
		return nil, fmt.Errorf("circuit breaker: half-open max requests must be >= 1")
	}
	name := s.Name
	if name == "" {
		name = "grinex"
	}
	thr := s.ConsecutiveFailures
	zl := log
	if zl == nil {
		zl = zap.NewNop()
	}
	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: s.HalfOpenMaxRequests,
		Interval:    s.Interval,
		Timeout:     s.OpenTimeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= thr
		},
		OnStateChange: func(cbName string, from gobreaker.State, to gobreaker.State) {
			zl.Info("circuit breaker state change",
				zap.String("name", cbName),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}
	return gobreaker.NewCircuitBreaker(st), nil
}
