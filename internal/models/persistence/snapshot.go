package persistence

import "time"

// Snapshot is the persistence layer row for rate_snapshots.
type Snapshot struct {
	ExchangeTime time.Time
	Bid          float64
	Ask          float64
	BidTopN      float64
	AskTopN      float64
	BidAvgNM     float64
	AskAvgNM     float64
	TopN         int32
	AvgN         int32
	AvgM         int32
}
