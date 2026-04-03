package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	ratesv1 "usdt-rates/gen/rates/v1"
	"usdt-rates/internal/application"
	"usdt-rates/internal/grpc/interceptor"
	"usdt-rates/internal/health"
	"usdt-rates/internal/server"
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

	lis, err := net.Listen("tcp", app.Config.GRPCAddr)
	if err != nil {
		app.Logger.Fatal("listen", zap.Error(err))
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptor.UnaryRequestLogger(app.Logger)),
		grpc.ChainStreamInterceptor(interceptor.StreamRequestLogger(app.Logger)),
	)
	ratesv1.RegisterRatesServiceServer(s, server.NewRates(app.Config, app.Grinex, app.PostgresRepo))

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
