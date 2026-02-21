package config

import (
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new logger instance based on configuration.
// When an OTel LoggerProvider is passed, logs are tee'd to both stdout and
// the OTel pipeline (which forwards to Loki via the Collector). The bridge
// automatically injects traceId/spanId into log records for correlation.
func NewLogger(cfg *Config, loggerProvider ...*sdklog.LoggerProvider) (*zap.Logger, error) {
	var level zapcore.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn", "warning":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.IsDevelopment() {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	stdoutCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Tee stdout + OTel bridge when a LoggerProvider is available
	core := stdoutCore
	if len(loggerProvider) > 0 && loggerProvider[0] != nil {
		otelCore := otelzap.NewCore("automatiza", otelzap.WithLoggerProvider(loggerProvider[0]))
		core = zapcore.NewTee(stdoutCore, otelCore)
	}

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// NewModuleLogger creates a logger for a specific module
func NewModuleLogger(logger *zap.Logger, module string) *zap.Logger {
	return logger.Named(module)
}
