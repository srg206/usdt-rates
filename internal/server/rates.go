package server

import (
	"context"

	"usdt-rates/config"
	ratesv1 "usdt-rates/gen/rates/v1"
	"usdt-rates/internal/calc"
	"usdt-rates/internal/grinex"
	"usdt-rates/internal/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Rates implements ratesv1.RatesServiceServer.
type Rates struct {
	ratesv1.UnimplementedRatesServiceServer
	cfg    *config.Config
	gx     *grinex.Client
	repo   *repository.Repository
}

// NewRates wires dependencies for the gRPC service.
func NewRates(cfg *config.Config, gx *grinex.Client, repo *repository.Repository) *Rates {
	return &Rates{cfg: cfg, gx: gx, repo: repo}
}

// GetRates fetches the order book, computes metrics, stores them, and returns the response.
func (s *Rates) GetRates(ctx context.Context, _ *ratesv1.GetRatesRequest) (*ratesv1.GetRatesResponse, error) {
	book, err := s.gx.Fetch(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "grinex: %v", err)
	}

	bid := book.Bids[0]
	ask := book.Asks[0]

	bidTop, err := calc.TopN(book.Bids, s.cfg.TopN)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}
	askTop, err := calc.TopN(book.Asks, s.cfg.TopN)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	bidAvg, err := calc.AvgNM(book.Bids, s.cfg.AvgN, s.cfg.AvgM)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}
	askAvg, err := calc.AvgNM(book.Asks, s.cfg.AvgN, s.cfg.AvgM)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	snap := repository.Snapshot{
		ExchangeTime: book.ExchangeTime,
		Bid:          bid,
		Ask:          ask,
		BidTopN:      bidTop,
		AskTopN:      askTop,
		BidAvgNM:     bidAvg,
		AskAvgNM:     askAvg,
		TopN:         int32(s.cfg.TopN),
		AvgN:         int32(s.cfg.AvgN),
		AvgM:         int32(s.cfg.AvgM),
	}
	if err := s.repo.InsertSnapshot(ctx, snap); err != nil {
		return nil, status.Errorf(codes.Internal, "db: %v", err)
	}

	return &ratesv1.GetRatesResponse{
		ExchangeTime: timestamppb.New(book.ExchangeTime),
		Bid:          bid,
		Ask:          ask,
		BidTopN:      bidTop,
		AskTopN:      askTop,
		BidAvgNm:     bidAvg,
		AskAvgNm:     askAvg,
		TopN:         int32(s.cfg.TopN),
		AvgN:         int32(s.cfg.AvgN),
		AvgM:         int32(s.cfg.AvgM),
	}, nil
}
