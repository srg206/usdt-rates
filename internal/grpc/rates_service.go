package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ratesv1 "usdt-rates/gen/rates/v1"
	apperrors "usdt-rates/internal/models/errors"
	"usdt-rates/internal/models/mappers"
	"usdt-rates/internal/usecase"
)

// RatesService implements ratesv1.RatesServiceServer by delegating to usecase.GetRates.
type RatesService struct {
	ratesv1.UnimplementedRatesServiceServer
	uc *usecase.GetRates
}

// NewRatesService builds the gRPC adapter.
func NewRatesService(uc *usecase.GetRates) *RatesService {
	return &RatesService{uc: uc}
}

// GetRates handles the RPC and maps use case errors to gRPC status codes.
func (s *RatesService) GetRates(ctx context.Context, _ *ratesv1.GetRatesRequest) (*ratesv1.GetRatesResponse, error) {
	snap, err := s.uc.Execute(ctx)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrOrderBook):
			return nil, status.Errorf(codes.Unavailable, "grinex: %v", err)
		case errors.Is(err, apperrors.ErrMetrics):
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		case errors.Is(err, apperrors.ErrPersist):
			return nil, status.Errorf(codes.Internal, "db: %v", err)
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return mappers.RateSnapshotToGetRatesResponse(snap), nil
}
