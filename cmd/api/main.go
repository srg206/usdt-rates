package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	grpcstd "google.golang.org/grpc"

	ratesv1 "usdt-rates/gen/rates/v1"
	"usdt-rates/internal/application"
	"usdt-rates/internal/grpc"
	"usdt-rates/internal/grpc/interceptor"
	"usdt-rates/internal/http/health"
	"usdt-rates/internal/infrastructure/metrics"
	"usdt-rates/internal/infrastructure/telemetry"
	"usdt-rates/internal/usecase"
)

func main() {
	if err := loadAppEnv(); err != nil {
		log.Fatalf("app.env: %v", err)
	}

	ctx := context.Background()
	app, err := application.NewApp(ctx)
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}

	if app.Config.OtelCollectorURL != "" {
		tp, tracerErr := telemetry.InitTracer(ctx, "usdt-rates", app.Config.OtelCollectorURL)
		if tracerErr != nil {
			app.Logger.Warn("failed to initialize otel tracer", zap.Error(tracerErr))
		} else {
			app.Closer.Add(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), app.Config.ShutdownTimeout)
				defer cancel()
				return tp.Shutdown(ctx)
			})
			app.Logger.Info("otel tracer initialized", zap.String("url", app.Config.OtelCollectorURL))
		}
	}

	lis, err := net.Listen("tcp", app.Config.GRPCAddr)
	if err != nil {
		app.Logger.Fatal("listen", zap.Error(err))
	}

	promMetrics := metrics.NewGRPCMetrics()
	if err := prometheus.Register(promMetrics); err != nil {
		app.Logger.Fatal("prometheus register grpc metrics", zap.Error(err))
	}

	s := grpcstd.NewServer(
		grpcstd.StatsHandler(otelgrpc.NewServerHandler()),
		grpcstd.ChainUnaryInterceptor(
			interceptor.UnaryRequestLogger(app.Logger),
			promMetrics.UnaryServerInterceptor(),
		),
		grpcstd.ChainStreamInterceptor(
			interceptor.StreamRequestLogger(app.Logger),
			promMetrics.StreamServerInterceptor(),
		),
	)
	getRates := usecase.NewGetRates(app.Config, app.Grinex, app.PostgresRepo)
	ratesv1.RegisterRatesServiceServer(s, grpc.NewRatesService(getRates))
	promMetrics.InitializeMetrics(s)

	app.Closer.Add(func() error {
		s.GracefulStop()
		return nil
	})

	go func() {
		app.Logger.Info("gRPC listening", zap.String("addr", app.Config.GRPCAddr))
		if err := s.Serve(lis); err != nil {
			app.Logger.Error("grpc serve", zap.Error(err))
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	health.NewHandler(app.PostgresRepo, app.Grinex, app.Config.HTTPTimeout).Mount(mux)
	healthSrv := &http.Server{Addr: app.Config.HealthHTTPAddr, Handler: mux}
	app.Closer.Add(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), app.Config.ShutdownTimeout)
		defer cancel()
		return healthSrv.Shutdown(ctx)
	})
	go func() {
		app.Logger.Info("health HTTP listening", zap.String("addr", app.Config.HealthHTTPAddr))
		if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Logger.Error("health http serve", zap.Error(err))
		}
	}()

	app.Closer.Wait()
}

func loadAppEnv() error {
	_, err := os.Stat("app.env")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return godotenv.Load("app.env")
}
