package usecase

import (
	"context"
	"fmt"

	"usdt-rates/config"
	"usdt-rates/internal/calc"
	"usdt-rates/internal/models/domain"
	"usdt-rates/internal/models/errors"
	"usdt-rates/internal/models/mappers"
)

// GetRates loads the order book, computes metrics, persists a snapshot, and returns the domain aggregate.
type GetRates struct {
	cfg  *config.Config
	gx   OrderBookFetcher
	repo RateSnapshotInserter
}

// NewGetRates wires dependencies for the GetRates scenario.
func NewGetRates(cfg *config.Config, gx OrderBookFetcher, repo RateSnapshotInserter) *GetRates {
	return &GetRates{cfg: cfg, gx: gx, repo: repo}
}

// Execute runs the full flow and returns the snapshot that was (or would be) exposed to clients.
func (u *GetRates) Execute(ctx context.Context) (domain.RateSnapshot, error) {
	book, err := u.gx.Fetch(ctx)
	if err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrOrderBook, err)
	}

	bid := book.Bids[0]
	ask := book.Asks[0]

	bidTop, err := calc.TopN(book.Bids, u.cfg.TopN)
	if err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrMetrics, err)
	}
	askTop, err := calc.TopN(book.Asks, u.cfg.TopN)
	if err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrMetrics, err)
	}

	bidAvg, err := calc.AvgNM(book.Bids, u.cfg.AvgN, u.cfg.AvgM)
	if err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrMetrics, err)
	}
	askAvg, err := calc.AvgNM(book.Asks, u.cfg.AvgN, u.cfg.AvgM)
	if err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrMetrics, err)
	}

	snap := domain.RateSnapshot{
		ExchangeTime: book.ExchangeTime,
		Bid:          bid,
		Ask:          ask,
		BidTopN:      bidTop,
		AskTopN:      askTop,
		BidAvgNM:     bidAvg,
		AskAvgNM:     askAvg,
		TopN:         int32(u.cfg.TopN),
		AvgN:         int32(u.cfg.AvgN),
		AvgM:         int32(u.cfg.AvgM),
	}
	if err := u.repo.InsertSnapshot(ctx, mappers.RateSnapshotToPersistence(snap)); err != nil {
		return domain.RateSnapshot{}, fmt.Errorf("%w: %v", apperrors.ErrPersist, err)
	}

	return snap, nil
}
