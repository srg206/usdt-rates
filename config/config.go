package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings loaded from environment variables with CLI flag overrides.
type Config struct {
	GRPCAddr             string
	HealthHTTPAddr       string
	PostgresURL          string
	MigrationsPath       string
	GrinexDepthURL       string
	HTTPTimeout          time.Duration
	TopN                 int
	AvgN                 int
	AvgM                 int
	ShutdownTimeout      time.Duration
	LogLevel             string
	LogFormat            string
	LogDisableStacktrace bool
}

// Load reads required environment variables, then applies flag overrides (flag.Parse).
func Load() (*Config, error) {
	grpcAddr, err := requireEnv("GRPC_ADDR")
	if err != nil {
		return nil, err
	}
	healthHTTPAddr, err := requireEnv("HEALTH_HTTP_ADDR")
	if err != nil {
		return nil, err
	}
	postgresURL, err := requireEnv("POSTGRES_URL")
	if err != nil {
		return nil, err
	}
	migrationsPath, err := requireEnv("MIGRATIONS_PATH")
	if err != nil {
		return nil, err
	}
	grinexDepthURL, err := requireEnv("GRINEX_DEPTH_URL")
	if err != nil {
		return nil, err
	}
	httpTimeout, err := requireDuration("HTTP_CLIENT_TIMEOUT")
	if err != nil {
		return nil, err
	}
	shutdownTimeout, err := requireDuration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return nil, err
	}
	topN, err := requireInt("CALC_TOP_N")
	if err != nil {
		return nil, err
	}
	avgN, err := requireInt("CALC_AVG_N")
	if err != nil {
		return nil, err
	}
	avgM, err := requireInt("CALC_AVG_M")
	if err != nil {
		return nil, err
	}
	logLevel, err := requireEnv("LOG_LEVEL")
	if err != nil {
		return nil, err
	}
	logFormat, err := requireEnv("LOG_FORMAT")
	if err != nil {
		return nil, err
	}
	logDisableStacktrace, err := requireBool("LOG_DISABLE_STACKTRACE")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		GRPCAddr:             grpcAddr,
		HealthHTTPAddr:       healthHTTPAddr,
		PostgresURL:          postgresURL,
		MigrationsPath:       migrationsPath,
		GrinexDepthURL:       grinexDepthURL,
		HTTPTimeout:          httpTimeout,
		ShutdownTimeout:      shutdownTimeout,
		TopN:                 topN,
		AvgN:                 avgN,
		AvgM:                 avgM,
		LogLevel:             logLevel,
		LogFormat:            logFormat,
		LogDisableStacktrace: logDisableStacktrace,
	}

	flag.StringVar(&cfg.GRPCAddr, "grpc-addr", cfg.GRPCAddr, "gRPC listen address")
	flag.StringVar(&cfg.HealthHTTPAddr, "health-http-addr", cfg.HealthHTTPAddr, "HTTP listen address for /healthz/* probes")
	flag.StringVar(&cfg.PostgresURL, "postgres-url", cfg.PostgresURL, "PostgreSQL connection URL")
	flag.StringVar(&cfg.MigrationsPath, "migrations-path", cfg.MigrationsPath, "goose migrations directory")
	flag.StringVar(&cfg.GrinexDepthURL, "grinex-depth-url", cfg.GrinexDepthURL, "Grinex HTTP depth/order book URL")
	flag.DurationVar(&cfg.HTTPTimeout, "http-timeout", cfg.HTTPTimeout, "HTTP client timeout")
	flag.IntVar(&cfg.TopN, "calc-top-n", cfg.TopN, "1-based order book index for topN price")
	flag.IntVar(&cfg.AvgN, "calc-avg-n", cfg.AvgN, "1-based start level for avgNM (inclusive)")
	flag.IntVar(&cfg.AvgM, "calc-avg-m", cfg.AvgM, "1-based end level for avgNM (inclusive)")
	flag.DurationVar(&cfg.ShutdownTimeout, "shutdown-timeout", cfg.ShutdownTimeout, "graceful shutdown timeout")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "zap log level (debug, info, warn, error, ...)")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "log format: json or console")
	flag.BoolVar(&cfg.LogDisableStacktrace, "log-disable-stacktrace", cfg.LogDisableStacktrace, "disable zap stack traces on warn/error")
	flag.Parse()

	if cfg.TopN < 1 || cfg.AvgN < 1 || cfg.AvgM < cfg.AvgN {
		return nil, fmt.Errorf("invalid calc bounds: top_n=%d avg_n=%d avg_m=%d", cfg.TopN, cfg.AvgN, cfg.AvgM)
	}
	return cfg, nil
}

func requireEnv(key string) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set or empty", key)
	}
	return v, nil
}

func requireInt(key string) (int, error) {
	s, err := requireEnv(key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q: invalid integer %q: %w", key, s, err)
	}
	return n, nil
}

func requireDuration(key string) (time.Duration, error) {
	s, err := requireEnv(key)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q: invalid duration %q: %w", key, s, err)
	}
	return d, nil
}

func requireBool(key string) (bool, error) {
	s, err := requireEnv(key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("environment variable %q: invalid bool %q: %w", key, s, err)
	}
	return b, nil
}
