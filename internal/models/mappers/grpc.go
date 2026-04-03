package mappers

import (
	ratesv1 "usdt-rates/gen/rates/v1"
	"usdt-rates/internal/models/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// RateSnapshotToGetRatesResponse maps domain aggregate to the gRPC transport message.
func RateSnapshotToGetRatesResponse(s domain.RateSnapshot) *ratesv1.GetRatesResponse {
	return &ratesv1.GetRatesResponse{
		ExchangeTime: timestamppb.New(s.ExchangeTime),
		Bid:          s.Bid,
		Ask:          s.Ask,
		BidTopN:      s.BidTopN,
		AskTopN:      s.AskTopN,
		BidAvgNm:     s.BidAvgNM,
		AskAvgNm:     s.AskAvgNM,
		TopN:         s.TopN,
		AvgN:         s.AvgN,
		AvgM:         s.AvgM,
	}
}
