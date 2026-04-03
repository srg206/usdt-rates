package apperrors

import "errors"

// Sentinel errors for mapping in the transport layer (e.g. gRPC codes).
var (
	ErrOrderBook = errors.New("order book")
	ErrMetrics   = errors.New("metrics")
	ErrPersist   = errors.New("persist")
)
