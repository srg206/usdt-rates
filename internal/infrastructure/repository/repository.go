package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"usdt-rates/internal/models/persistence"
)

// Repository stores rate snapshots in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a repository backed by a pgx pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertSnapshot persists a quote row.
func (r *Repository) InsertSnapshot(ctx context.Context, s persistence.Snapshot) error {
	const q = `
INSERT INTO rate_snapshots (
	exchange_time, bid, ask, bid_top_n, ask_top_n, bid_avg_nm, ask_avg_nm, top_n, avg_n, avg_m
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err := r.pool.Exec(ctx, q,
		s.ExchangeTime, s.Bid, s.Ask, s.BidTopN, s.AskTopN, s.BidAvgNM, s.AskAvgNM, s.TopN, s.AvgN, s.AvgM)
	return err
}

// Ping checks database connectivity.
func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
