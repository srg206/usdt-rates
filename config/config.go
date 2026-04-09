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
	GrinexWarmupURL      string
	GrinexJSONOrigin     string
	GrinexJSONReferer    string
	GrinexMaxConcurrent  int
	GrinexCBEnabled      bool
	GrinexCBFailures     int
	GrinexCBOpenTimeout  time.Duration
	GrinexCBHalfOpenMax  int
	GrinexCBInterval     time.Duration
	HTTPTimeout          time.Duration
	TopN                 int
	AvgN                 int
	AvgM                 int
	ShutdownTimeout      time.Duration
	LogLevel             string
	LogFormat            string
	LogDisableStacktrace bool
	OtelCollectorURL     string
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
	grinexWarmupURL, err := requireEnv("GRINEX_WARMUP_URL")
	if err != nil {
		return nil, err
	}
	grinexJSONOrigin, err := requireEnv("GRINEX_JSON_ORIGIN")
	if err != nil {
		return nil, err
	}
	grinexJSONReferer, err := requireEnv("GRINEX_JSON_REFERER")
	if err != nil {
		return nil, err
	}
	grinexMaxConcurrent, err := requireInt("GRINEX_MAX_CONCURRENT")
	if err != nil {
		return nil, err
	}
	grinexCBEnabled, err := requireBool("GRINEX_CB_ENABLED")
	if err != nil {
		return nil, err
	}
	grinexCBFailures, err := requireInt("GRINEX_CB_CONSECUTIVE_FAILURES")
	if err != nil {
		return nil, err
	}
	grinexCBOpenTimeout, err := requireDuration("GRINEX_CB_OPEN_TIMEOUT")
	if err != nil {
		return nil, err
	}
	grinexCBHalfOpenMax, err := requireInt("GRINEX_CB_HALF_OPEN_MAX")
	if err != nil {
		return nil, err
	}
	grinexCBInterval, err := requireDuration("GRINEX_CB_INTERVAL")
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
	otelCollectorURL := strings.TrimSpace(os.Getenv("OTEL_COLLECTOR_URL"))

	cfg := &Config{
		GRPCAddr:             grpcAddr,
		HealthHTTPAddr:       healthHTTPAddr,
		PostgresURL:          postgresURL,
		MigrationsPath:       migrationsPath,
		GrinexDepthURL:       grinexDepthURL,
		GrinexWarmupURL:      grinexWarmupURL,
		GrinexJSONOrigin:     grinexJSONOrigin,
		GrinexJSONReferer:    grinexJSONReferer,
		GrinexMaxConcurrent:  grinexMaxConcurrent,
		GrinexCBEnabled:      grinexCBEnabled,
		GrinexCBFailures:     grinexCBFailures,
		GrinexCBOpenTimeout:  grinexCBOpenTimeout,
		GrinexCBHalfOpenMax:  grinexCBHalfOpenMax,
		GrinexCBInterval:     grinexCBInterval,
		HTTPTimeout:          httpTimeout,
		ShutdownTimeout:      shutdownTimeout,
		TopN:                 topN,
		AvgN:                 avgN,
		AvgM:                 avgM,
		LogLevel:             logLevel,
		LogFormat:            logFormat,
		LogDisableStacktrace: logDisableStacktrace,
		OtelCollectorURL:     otelCollectorURL,
	}

	flag.StringVar(&cfg.GRPCAddr, "grpc-addr", cfg.GRPCAddr, "gRPC listen address")
	flag.StringVar(&cfg.HealthHTTPAddr, "health-http-addr", cfg.HealthHTTPAddr, "HTTP listen address for /healthz/* probes")
	flag.StringVar(&cfg.PostgresURL, "postgres-url", cfg.PostgresURL, "PostgreSQL connection URL")
	flag.StringVar(&cfg.MigrationsPath, "migrations-path", cfg.MigrationsPath, "goose migrations directory")
	flag.StringVar(&cfg.GrinexDepthURL, "grinex-depth-url", cfg.GrinexDepthURL, "Grinex HTTP depth/order book URL")
	flag.StringVar(&cfg.GrinexWarmupURL, "grinex-warmup-url", cfg.GrinexWarmupURL, "Grinex warmup GET URL (HTML, cookies / WAF)")
	flag.StringVar(&cfg.GrinexJSONOrigin, "grinex-json-origin", cfg.GrinexJSONOrigin, "Origin header for Grinex depth JSON request")
	flag.StringVar(&cfg.GrinexJSONReferer, "grinex-json-referer", cfg.GrinexJSONReferer, "Referer header for Grinex depth JSON request")
	flag.IntVar(&cfg.GrinexMaxConcurrent, "grinex-max-concurrent", cfg.GrinexMaxConcurrent, "max concurrent Grinex HTTP fetch operations (semaphore)")
	flag.BoolVar(&cfg.GrinexCBEnabled, "grinex-cb-enabled", cfg.GrinexCBEnabled, "enable circuit breaker for Grinex HTTP")
	flag.IntVar(&cfg.GrinexCBFailures, "grinex-cb-consecutive-failures", cfg.GrinexCBFailures, "trip breaker after this many consecutive Grinex failures")
	flag.DurationVar(&cfg.GrinexCBOpenTimeout, "grinex-cb-open-timeout", cfg.GrinexCBOpenTimeout, "how long breaker stays open before half-open trial")
	flag.IntVar(&cfg.GrinexCBHalfOpenMax, "grinex-cb-half-open-max", cfg.GrinexCBHalfOpenMax, "max calls in half-open state")
	flag.DurationVar(&cfg.GrinexCBInterval, "grinex-cb-interval", cfg.GrinexCBInterval, "closed-state window to reset failure counts (0 = never reset)")
	flag.DurationVar(&cfg.HTTPTimeout, "http-timeout", cfg.HTTPTimeout, "HTTP client timeout")
	flag.IntVar(&cfg.TopN, "calc-top-n", cfg.TopN, "1-based order book index for topN price")
	flag.IntVar(&cfg.AvgN, "calc-avg-n", cfg.AvgN, "1-based start level for avgNM (inclusive)")
	flag.IntVar(&cfg.AvgM, "calc-avg-m", cfg.AvgM, "1-based end level for avgNM (inclusive)")
	flag.DurationVar(&cfg.ShutdownTimeout, "shutdown-timeout", cfg.ShutdownTimeout, "graceful shutdown timeout")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "zap log level (debug, info, warn, error, ...)")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "log format: json or console")
	flag.BoolVar(&cfg.LogDisableStacktrace, "log-disable-stacktrace", cfg.LogDisableStacktrace, "disable zap stack traces on warn/error")
	flag.StringVar(&cfg.OtelCollectorURL, "otel-collector-url", cfg.OtelCollectorURL, "OpenTelemetry OTLP HTTP endpoint (host:port)")
	flag.Parse()

	if cfg.TopN < 1 || cfg.AvgN < 1 || cfg.AvgM < cfg.AvgN {
		return nil, fmt.Errorf("invalid calc bounds: top_n=%d avg_n=%d avg_m=%d", cfg.TopN, cfg.AvgN, cfg.AvgM)
	}
	if cfg.GrinexMaxConcurrent < 1 {
		return nil, fmt.Errorf("GRINEX_MAX_CONCURRENT must be >= 1, got %d", cfg.GrinexMaxConcurrent)
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
