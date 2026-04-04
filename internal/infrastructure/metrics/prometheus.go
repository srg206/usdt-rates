package metrics

import (
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
)

// NewGRPCMetrics returns gRPC Prometheus metrics with handling-time histogram (for p50/p95/p99 in Grafana).
func NewGRPCMetrics() *prometheus.ServerMetrics {
	return prometheus.NewServerMetrics(prometheus.WithServerHandlingTimeHistogram())
}
