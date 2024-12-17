package logging

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MockLogger is a simple implementation of Logger for testing purposes
type MockLogger struct{}

// Log is a no-op implementation of the Logger interface
func (m *MockLogger) Log(level string, format string, args ...interface{}) {
	// No-op
}

func TestGetLogLevel(t *testing.T) {
	// Test valid log levels
	testCases := []struct {
		logLevel    string
		expectError bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"panic", false},
		{"dpanic", false},
		{"invalid", true}, // Invalid log level
		{"", true},        // Empty string should trigger an error
	}

	for _, tc := range testCases {
		t.Run(tc.logLevel, func(t *testing.T) {
			err := InitLogger(tc.logLevel)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.logLevel, GetLogLevel())
			}
		})
	}

	// Test the "unknown" log level
	t.Run("UnknownLogLevel", func(t *testing.T) {
		// Assign a MockLogger to the global logger
		globalLogger = &MockLogger{}

		// Assert that GetLogLevel returns "unknown"
		assert.Equal(t, "unknown", GetLogLevel())
	})
}

func TestNewLogger_BuildFailure(t *testing.T) {
	// Create a faulty configuration with an invalid output path
	faultyConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Development: false,
		Encoding:    "console",                     // Valid encoding
		OutputPaths: []string{"/dev/null/invalid"}, // Guaranteed invalid path
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

	// Attempt to create a logger with the faulty config
	logger, err := NewLoggerWithConfig("debug", faultyConfig)

	// Assert that the logger is nil and an error is returned
	assert.Nil(t, logger, "Logger should be nil on build failure")
	assert.Error(t, err, "Expected an error when building logger with faulty config")
	assert.Contains(t, err.Error(), "open", "Error message should indicate the cause of failure")
}

func TestLogMessage(t *testing.T) {
	t.Run("UninitializedLogger", func(t *testing.T) {
		// Temporarily redirect stderr to capture output
		r, w, err := os.Pipe()
		assert.NoError(t, err)

		originalStderr := os.Stderr
		os.Stderr = w

		// Ensure global logger is nil
		globalLogger = nil

		// Write the log message
		expectedMessage := "Global logger not initialized. Message: Uninitialized logger test"
		LogMessage("info", "Uninitialized logger test")

		// Close the write end of the pipe to flush output
		assert.NoError(t, w.Close())
		os.Stderr = originalStderr

		// Read from the pipe
		var buf bytes.Buffer
		_, err = buf.ReadFrom(r)
		assert.NoError(t, err)

		// Close the read end
		assert.NoError(t, r.Close())

		// Assert the output
		capturedOutput := buf.String()
		assert.Contains(t, capturedOutput, expectedMessage)
	})

	t.Run("LogLevelsExcludingErrorAndHigher", func(t *testing.T) {
		// Set up a new logger
		assert.NoError(t, InitLogger("debug"))

		logLevels := []string{"debug", "info", "warn", "error"}
		for _, level := range logLevels {
			t.Run(fmt.Sprintf("LogLevel:%s", level), func(t *testing.T) {
				assert.NotPanics(t, func() {
					LogMessage(level, "This is a %s test message", level)
				})
			})
		}
	})

	t.Run("LogLevelPanic", func(t *testing.T) {
		// Set up a new logger
		assert.NoError(t, InitLogger("debug"))

		assert.Panics(t, func() {
			LogMessage("panic", "This is a panic test message")
		})
	})

	t.Run("LogLevelDpanic", func(t *testing.T) {
		// Set Development: true to trigger a panic for "dpanic"
		config := zap.Config{
			Level:       zap.NewAtomicLevelAt(zapcore.DebugLevel),
			Development: true, // Development mode
			Encoding:    "console",
			OutputPaths: []string{"stdout"},
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:       "ts",
				LevelKey:      "level",
				CallerKey:     "caller",
				MessageKey:    "msg",
				StacktraceKey: "stacktrace",
				LineEnding:    zapcore.DefaultLineEnding,
				EncodeLevel:   zapcore.CapitalColorLevelEncoder,
				EncodeTime:    zapcore.ISO8601TimeEncoder,
				EncodeCaller:  zapcore.ShortCallerEncoder,
			},
		}

		logger, err := config.Build()
		assert.NoError(t, err)

		// Inject the logger with Development: true
		SetCustomLogger(logger.Sugar())
		defer func() {
			if err := logger.Sync(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
			}
		}()

		// Expect a panic in Development mode
		assert.Panics(t, func() {
			LogMessage("dpanic", "This is a dpanic test message")
		})
	})

	t.Run("UnknownLogLevel", func(t *testing.T) {
		// Set up a new logger
		assert.NoError(t, InitLogger("info"))
		assert.NotPanics(t, func() {
			LogMessage("unknown", "This should log with an error about the level")
		})
	})
}

func TestLogMessage_UninitializedRealLogger(t *testing.T) {
	// Temporarily redirect stderr to capture output
	r, w, err := os.Pipe()
	assert.NoError(t, err)

	originalStderr := os.Stderr
	os.Stderr = w

	// Create a RealLogger instance with nil sugar
	logger := &RealLogger{sugar: nil}

	// Log a message
	expectedMessage := "Logger not initialized. Message: Uninitialized RealLogger test"
	logger.Log("info", "Uninitialized RealLogger test")

	// Close the write end of the pipe to flush output
	assert.NoError(t, w.Close())
	os.Stderr = originalStderr

	// Read from the pipe
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	assert.NoError(t, err)

	// Close the read end
	assert.NoError(t, r.Close())

	// Assert the output
	capturedOutput := buf.String()
	assert.Contains(t, capturedOutput, expectedMessage)
}

func TestGetGlobalLogger(t *testing.T) {
	t.Run("ReturnsNoOpLoggerWhenGlobalLoggerUninitialized", func(t *testing.T) {
		// Explicitly unset the global logger
		globalLogger = nil

		logger := GetGlobalLogger()

		// Assert that the returned logger is a NoOpLogger
		_, isNoOp := logger.(*NoOpLogger)
		assert.True(t, isNoOp, "Expected NoOpLogger as fallback for uninitialized globalLogger")

		// Redirect stderr to capture the output
		r, w, err := os.Pipe()
		assert.NoError(t, err)

		originalStderr := os.Stderr
		os.Stderr = w

		// Call the function to trigger the Fprintln message
		_ = GetGlobalLogger()

		// Close the write end to flush output
		assert.NoError(t, w.Close())
		os.Stderr = originalStderr

		// Read from the pipe
		var buf bytes.Buffer
		_, err = buf.ReadFrom(r)
		assert.NoError(t, err)

		// Close the read end
		assert.NoError(t, r.Close())

		// Assert the stderr message
		assert.Contains(t, buf.String(), "Global logger is uninitialized. Returning no-op logger.")
	})

	t.Run("ReturnsGlobalLoggerWhenInitialized", func(t *testing.T) {
		// Initialize the global logger
		err := InitLogger("info")
		assert.NoError(t, err)

		logger := GetGlobalLogger()

		// Assert that the returned logger is the initialized RealLogger
		_, isRealLogger := logger.(*RealLogger)
		assert.True(t, isRealLogger, "Expected RealLogger when globalLogger is initialized")
	})
}
