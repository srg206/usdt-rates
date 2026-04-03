package domain

import "time"

// RateSnapshot is the domain aggregate after metrics are computed from an order book.
type RateSnapshot struct {
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
