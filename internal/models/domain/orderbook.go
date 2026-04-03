package domain

import "time"

// OrderBook is the domain view of a bid/ask ladder (best level first per side).
type OrderBook struct {
	Bids          []float64
	Asks          []float64
	ExchangeTime  time.Time
	HasServerTime bool
}
