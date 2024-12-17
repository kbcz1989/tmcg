// Package logging provides utility functions for structured and leveled logging
// throughout the tmcg application.
package logging

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines an interface for logging
type Logger interface {
	Log(level string, format string, args ...interface{})
}

// RealLogger is the production implementation of Logger
type RealLogger struct {
	sugar    *zap.SugaredLogger
	logLevel zapcore.Level
}

// InitLogger initializes a RealLogger with the specified log level and sets it globally
func InitLogger(level string) error {
	realLogger, err := NewLogger(level)
	if err != nil {
		return err
	}

	SetGlobalLogger(realLogger)
	return nil
}

func NewLogger(level string) (*RealLogger, error) {
	defaultConfig := zap.Config{
		Development: false,
		Encoding:    "console",
		OutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:      "ts",
			LevelKey:     "level",
			CallerKey:    "caller",
			MessageKey:   "msg",
			EncodeLevel:  zapcore.CapitalColorLevelEncoder,
			EncodeTime:   zapcore.ISO8601TimeEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}
	return NewLoggerWithConfig(level, defaultConfig)
}

// NewLogger creates a new RealLogger instance
func NewLoggerWithConfig(level string, config zap.Config) (*RealLogger, error) {
	// Parse log level
	var lvl zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info":
		lvl = zapcore.InfoLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	case "panic":
		lvl = zapcore.PanicLevel
	case "dpanic":
		lvl = zapcore.DPanicLevel
	default:
		return nil, fmt.Errorf("invalid log level: %s", level)
	}

	config.Level = zap.NewAtomicLevelAt(lvl)

	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.PanicLevel),
	)
	if err != nil {
		return nil, err
	}

	return &RealLogger{
		sugar:    logger.Sugar(),
		logLevel: lvl,
	}, nil
}

// Log logs a message using the specified log level
func (r *RealLogger) Log(level string, format string, args ...interface{}) {
	if r.sugar == nil {
		fmt.Fprintf(os.Stderr, "Logger not initialized. Message: "+format+"\n", args...)
		return
	}

	switch strings.ToLower(level) {
	case "debug":
		r.sugar.Debugf(format, args...)
	case "info":
		r.sugar.Infof(format, args...)
	case "warn":
		r.sugar.Warnf(format, args...)
	case "error":
		r.sugar.Errorf(format, args...)
	case "panic":
		r.sugar.Panicf(format, args...)
	case "dpanic":
		r.sugar.DPanicf(format, args...)
	default:
		r.sugar.Infof(format, args...) // Default to InfoLevel if an unknown level is provided
	}
}

// Global logger for backward compatibility
var globalLogger Logger

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// LogMessage logs messages using the global logger for backward compatibility
func LogMessage(level string, format string, args ...interface{}) {
	if globalLogger == nil {
		fmt.Fprintf(os.Stderr, "Global logger not initialized. Message: "+format+"\n", args...)
		return
	}
	globalLogger.Log(level, format, args...)
}

// GetLogLevel returns the current log level of the global logger
func GetLogLevel() string {
	if realLogger, ok := globalLogger.(*RealLogger); ok {
		return realLogger.logLevel.String()
	}
	return "unknown"
}

// SetCustomLogger allows injecting a custom SugaredLogger for testing purposes
func SetCustomLogger(customLogger *zap.SugaredLogger) {
	globalLogger = &RealLogger{
		sugar: customLogger,
	}
}

// GetGlobalLogger returns the global logger instance, or a default no-op logger if not initialized.
func GetGlobalLogger() Logger {
	if globalLogger == nil {
		fmt.Fprintln(os.Stderr, "Global logger is uninitialized. Returning no-op logger.")
		return &NoOpLogger{}
	}
	return globalLogger
}

// NoOpLogger is a no-operation implementation of Logger for fallback purposes.
type NoOpLogger struct{}

// Log performs no action for NoOpLogger.
func (n *NoOpLogger) Log(level string, format string, args ...interface{}) {
	// No-op
}
