package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"usdt-rates/config"
	"usdt-rates/internal/models/domain"
	apperrors "usdt-rates/internal/models/errors"
	"usdt-rates/internal/models/persistence"
	"usdt-rates/internal/usecase"
)

type mockOrderBookFetcher struct {
	book domain.OrderBook
	err  error
}

func (m *mockOrderBookFetcher) Fetch(ctx context.Context) (domain.OrderBook, error) {
	return m.book, m.err
}

type mockRateSnapshotInserter struct {
	insertErr error
	inserted  []persistence.Snapshot
}

func (m *mockRateSnapshotInserter) InsertSnapshot(ctx context.Context, s persistence.Snapshot) error {
	if m.insertErr != nil {
		return m.insertErr
	}
	m.inserted = append(m.inserted, s)
	return nil
}

func TestGetRates_Execute_success(t *testing.T) {
	t.Parallel()

	wantTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	cfg := &config.Config{
		TopN: 2,
		AvgN: 1,
		AvgM: 2,
	}
	book := domain.OrderBook{
		Bids:         []float64{100, 99, 98},
		Asks:         []float64{101, 102, 103},
		ExchangeTime: wantTime,
	}
	gx := &mockOrderBookFetcher{book: book}
	repo := &mockRateSnapshotInserter{}

	uc := usecase.NewGetRates(cfg, gx, repo)
	snap, err := uc.Execute(context.Background())
	require.NoError(t, err)

	require.Equal(t, wantTime, snap.ExchangeTime)
	require.Equal(t, 100.0, snap.Bid)
	require.Equal(t, 101.0, snap.Ask)
	require.Equal(t, 99.0, snap.BidTopN)
	require.Equal(t, 102.0, snap.AskTopN)
	require.InDelta(t, 99.5, snap.BidAvgNM, 1e-9)
	require.InDelta(t, 101.5, snap.AskAvgNM, 1e-9)
	require.Equal(t, int32(2), snap.TopN)
	require.Equal(t, int32(1), snap.AvgN)
	require.Equal(t, int32(2), snap.AvgM)

	require.Len(t, repo.inserted, 1)
	row := repo.inserted[0]
	require.Equal(t, wantTime, row.ExchangeTime)
	require.Equal(t, 100.0, row.Bid)
	require.Equal(t, 101.0, row.Ask)
	require.Equal(t, int32(2), row.TopN)
	require.Equal(t, int32(1), row.AvgN)
	require.Equal(t, int32(2), row.AvgM)
}

func TestGetRates_Execute_fetchError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{TopN: 1, AvgN: 1, AvgM: 1}
	gx := &mockOrderBookFetcher{err: context.Canceled}
	repo := &mockRateSnapshotInserter{}

	uc := usecase.NewGetRates(cfg, gx, repo)
	_, err := uc.Execute(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, apperrors.ErrOrderBook))
	require.Empty(t, repo.inserted)
}

func TestGetRates_Execute_metricsError(t *testing.T) {
	t.Parallel()

	// TopN=5 but the book only has 3 levels — calc.TopN returns an error.
	cfg := &config.Config{TopN: 5, AvgN: 1, AvgM: 5}
	book := domain.OrderBook{
		Bids:         []float64{100, 99, 98},
		Asks:         []float64{101, 102, 103},
		ExchangeTime: time.Now(),
	}
	gx := &mockOrderBookFetcher{book: book}
	repo := &mockRateSnapshotInserter{}

	uc := usecase.NewGetRates(cfg, gx, repo)
	_, err := uc.Execute(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, apperrors.ErrMetrics))
	require.Empty(t, repo.inserted)
}

func TestGetRates_Execute_persistError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{TopN: 1, AvgN: 1, AvgM: 1}
	book := domain.OrderBook{
		Bids:         []float64{100},
		Asks:         []float64{101},
		ExchangeTime: time.Now(),
	}
	gx := &mockOrderBookFetcher{book: book}
	repo := &mockRateSnapshotInserter{insertErr: errors.New("db unavailable")}

	uc := usecase.NewGetRates(cfg, gx, repo)
	_, err := uc.Execute(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, apperrors.ErrPersist))
}
