package mappers

import (
	"usdt-rates/internal/models/domain"
	"usdt-rates/internal/models/persistence"
)

// RateSnapshotToPersistence maps domain aggregate to the DB row shape.
func RateSnapshotToPersistence(s domain.RateSnapshot) persistence.Snapshot {
	return persistence.Snapshot{
		ExchangeTime: s.ExchangeTime,
		Bid:          s.Bid,
		Ask:          s.Ask,
		BidTopN:      s.BidTopN,
		AskTopN:      s.AskTopN,
		BidAvgNM:     s.BidAvgNM,
		AskAvgNM:     s.AskAvgNM,
		TopN:         s.TopN,
		AvgN:         s.AvgN,
		AvgM:         s.AvgM,
	}
}
