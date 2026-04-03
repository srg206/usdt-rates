package application

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"usdt-rates/config"
	"usdt-rates/internal/grinex"
	"usdt-rates/internal/repository"
	"usdt-rates/pkg/closer"
	"usdt-rates/pkg/logger"
)

// App holds infrastructure dependencies and lifecycle helpers.
type App struct {
	Config       *config.Config
	Logger       *zap.Logger
	Closer       *closer.Closer
	DB           *pgxpool.Pool
	Grinex       *grinex.Client
	PostgresRepo *repository.Repository
}

// NewApp loads config, opens Postgres, wires clients, registers pool cleanup on Closer.
func NewApp(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	app := &App{
		Config: cfg,
		Closer: closer.New(cfg.ShutdownTimeout),
	}

	zl, err := logger.New(cfg)
	if err != nil {
		return nil, err
	}
	app.Logger = zl
	app.Closer.Add(func() error {
		_ = zl.Sync()
		return nil
	})

	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	app.DB = pool
	app.PostgresRepo = repository.New(pool)

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	app.Closer.Add(func() error {
		pool.Close()
		return nil
	})

	gx, err := grinex.NewClient(cfg.HTTPTimeout, cfg.GrinexDepthURL, zl)
	if err != nil {
		return nil, fmt.Errorf("grinex: %w", err)
	}
	app.Grinex = gx

	return app, nil
}
