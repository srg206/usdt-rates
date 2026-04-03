package usecase

import (
	"context"

	"usdt-rates/internal/models/domain"
	"usdt-rates/internal/models/persistence"
)

// OrderBookFetcher loads the current order book from the exchange.
type OrderBookFetcher interface {
	Fetch(ctx context.Context) (domain.OrderBook, error)
}

// RateSnapshotInserter persists computed rate snapshots.
type RateSnapshotInserter interface {
	InsertSnapshot(ctx context.Context, s persistence.Snapshot) error
}
