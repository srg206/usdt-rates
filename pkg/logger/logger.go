package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"usdt-rates/config"
)

func New(cfg *config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		return nil, fmt.Errorf("LOG_LEVEL: %w", err)
	}

	var zcfg zap.Config
	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		zcfg = zap.NewProductionConfig()
	case "console":
		zcfg = zap.NewDevelopmentConfig()
	default:
		return nil, fmt.Errorf("LOG_FORMAT must be json or console, got %q", cfg.LogFormat)
	}

	zcfg.Level = zap.NewAtomicLevelAt(level)
	zcfg.DisableStacktrace = cfg.LogDisableStacktrace
	return zcfg.Build()
}
