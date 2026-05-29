package main

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var appLog = zap.NewNop().Sugar()

func initLogger() error {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_PRETTY")), "true") {
		cfg = zap.NewDevelopmentConfig()
	}
	if levelText := strings.TrimSpace(os.Getenv("LOG_LEVEL")); levelText != "" {
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(levelText)); err == nil {
			cfg.Level = zap.NewAtomicLevelAt(level)
		}
	}
	logger, err := cfg.Build()
	if err != nil {
		return err
	}
	appLog = logger.Sugar()
	return nil
}

func syncLogger() {
	_ = appLog.Sync()
}
