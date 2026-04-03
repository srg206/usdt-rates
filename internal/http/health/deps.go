package health

import (
	"context"

	"usdt-rates/internal/models/domain"
)

// DatabasePinger checks PostgreSQL connectivity (readiness only).
type DatabasePinger interface {
	Ping(ctx context.Context) error
}

// ExchangeDepthReadiness verifies the exchange depth endpoint responds (readiness only).
// Matches the exchange client’s Fetch signature so no adapter is needed in main.
type ExchangeDepthReadiness interface {
	Fetch(ctx context.Context) (domain.OrderBook, error)
}
