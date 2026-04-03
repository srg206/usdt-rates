package interceptor

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryRequestLogger logs each unary RPC (method, duration, gRPC code, error).
func UnaryRequestLogger(l *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		l.Info("grpc request", zap.String("method", info.FullMethod))

		defer func() {
			dur := time.Since(start)
			if err != nil {
				st, _ := status.FromError(err)
				l.Warn("grpc request failed",
					zap.String("method", info.FullMethod),
					zap.Duration("duration", dur),
					zap.String("grpc_code", st.Code().String()),
					zap.Error(err),
				)
				return
			}
			l.Info("grpc request completed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", dur),
			)
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}

// StreamRequestLogger logs stream RPC start and outcome (middleware-style).
func StreamRequestLogger(l *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		l.Info("grpc stream", zap.String("method", info.FullMethod))

		err := handler(srv, ss)
		dur := time.Since(start)
		if err != nil {
			st, _ := status.FromError(err)
			l.Warn("grpc stream failed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", dur),
				zap.String("grpc_code", st.Code().String()),
				zap.Error(err),
			)
			return err
		}
		l.Info("grpc stream completed",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", dur),
		)
		return nil
	}
}
