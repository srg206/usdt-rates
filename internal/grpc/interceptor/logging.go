package interceptor

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func traceFields(ctx context.Context) []zap.Field {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	}
}

// UnaryRequestLogger logs each unary RPC result (method, duration, gRPC code, error).
// Recovers from handler panics, converts them to Internal errors, and logs at Error level.
func UnaryRequestLogger(l *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "panic: %v", r)
				dur := time.Since(start)
				fields := append(traceFields(ctx),
					zap.String("method", info.FullMethod),
					zap.Duration("duration", dur),
					zap.Any("panic", r),
				)
				l.Error("grpc request panic", fields...)
				return
			}

			dur := time.Since(start)
			fields := append(traceFields(ctx),
				zap.String("method", info.FullMethod),
				zap.Duration("duration", dur),
			)
			if err != nil {
				st, _ := status.FromError(err)
				fields = append(fields,
					zap.String("grpc_code", st.Code().String()),
					zap.Error(err),
				)
				l.Warn("grpc request failed", fields...)
				return
			}
			l.Info("grpc request completed", fields...)
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}

// StreamRequestLogger logs stream RPC outcome (middleware-style).
// Recovers from handler panics and converts them to Internal errors.
func StreamRequestLogger(l *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "panic: %v", r)
				dur := time.Since(start)
				fields := append(traceFields(ss.Context()),
					zap.String("method", info.FullMethod),
					zap.Duration("duration", dur),
					zap.String("panic", fmt.Sprint(r)),
				)
				l.Error("grpc stream panic", fields...)
				return
			}

			dur := time.Since(start)
			fields := append(traceFields(ss.Context()),
				zap.String("method", info.FullMethod),
				zap.Duration("duration", dur),
			)
			if err != nil {
				st, _ := status.FromError(err)
				fields = append(fields,
					zap.String("grpc_code", st.Code().String()),
					zap.Error(err),
				)
				l.Warn("grpc stream failed", fields...)
				return
			}
			l.Info("grpc stream completed", fields...)
		}()

		err = handler(srv, ss)
		return err
	}
}
