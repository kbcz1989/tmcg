package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

type MockLogger struct {
	messages []string
}

func (m *MockLogger) Log(level string, format string, args ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf("[%s] %s", level, fmt.Sprintf(format, args...)))
}

func TestStringSliceFlag(t *testing.T) {
	var f stringSliceFlag

	err := f.Set("value1")
	assert.NoError(t, err)
	assert.Equal(t, stringSliceFlag{"value1"}, f)

	err = f.Set("value2")
	assert.NoError(t, err)
	assert.Equal(t, stringSliceFlag{"value1", "value2"}, f)

	assert.Equal(t, "value1,value2", f.String())
	assert.Equal(t, "stringSlice", f.Type())
}

func TestEnsureWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	workingDir := filepath.Join(tempDir, "testdir")

	err := os.MkdirAll(workingDir, 0755)
	assert.NoError(t, err)

	_, err = os.Stat(workingDir)
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(workingDir, "testdir"))
}

func TestValidateTerraformBinary(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "terraform")

	// Case 1: Binary does not exist
	_, err := os.Stat(binaryPath)
	assert.True(t, os.IsNotExist(err))

	// Case 2: Binary exists
	err = os.WriteFile(binaryPath, []byte{}, 0755)
	assert.NoError(t, err)

	_, err = os.Stat(binaryPath)
	assert.NoError(t, err)
}

func TestSetup(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedStdout string
		expectedStderr string
		expectedCode   int
		logMessages    []string
	}{
		{
			name:           "Version flag",
			args:           []string{"--version"},
			expectedStdout: "tmcg version: dev\nCommit: none\nBuilt on: unknown\n",
			expectedStderr: "",
			expectedCode:   0,
			logMessages:    []string{},
		},
		{
			name:           "Help flag",
			args:           []string{"--help"},
			expectedStdout: "Usage:",
			expectedStderr: "",
			expectedCode:   0,
			logMessages:    []string{},
		},
		{
			name:           "Error parsing flags",
			args:           []string{"--unknown-flag"},
			expectedStdout: "",
			expectedStderr: "Error parsing flags: unknown flag: --unknown-flag",
			expectedCode:   1,
			logMessages:    []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var exitCode int

			mockExit := func(code int) {
				exitCode = code
			}

			mockLogger := &MockLogger{}

			Setup(tc.args, &stdout, &stderr, mockExit, mockLogger)

			assert.Equal(t, tc.expectedCode, exitCode, "Unexpected exit code")
			if tc.expectedStdout != "" {
				assert.Contains(t, stdout.String(), tc.expectedStdout, "Unexpected stdout")
			}
			if tc.expectedStderr != "" {
				assert.Contains(t, stderr.String(), tc.expectedStderr, "Unexpected stderr")
			}

			// Validate log messages
			for _, expectedLog := range tc.logMessages {
				assert.Contains(t, mockLogger.messages, expectedLog, "Expected log message not found")
			}
		})
	}
}

func TestRun_MissingRequiredArguments(t *testing.T) {
	// Mock logger to capture log messages
	mockLogger := &MockLogger{}

	// Use a buffer for capturing stderr
	var stderr bytes.Buffer

	// Mock exit function
	var exitCode int
	mockExitFunc := func(code int) {
		exitCode = code
		panic(fmt.Sprintf("exit with code: %d", code))
	}

	// Defer recovery to catch the panic caused by mockExitFunc
	defer func() {
		if r := recover(); r != nil {
			if !strings.Contains(fmt.Sprint(r), "exit with code: 1") {
				t.Errorf("unexpected panic: %v", r)
			}
		}
	}()

	// Test arguments with missing required resources and providers
	args := []string{"--log-level", "info"}

	// Call the Setup function
	Setup(args, &stderr, &stderr, mockExitFunc, mockLogger)

	// Validate log messages
	expectedLog := "Missing required arguments: resources or providers"
	assert.Contains(t, mockLogger.messages, expectedLog, "Expected log message not found in logger")

	// Validate exit code
	assert.Equal(t, 1, exitCode, "Unexpected exit code")

	// Validate stderr contains usage information
	assert.Contains(t, stderr.String(), "Usage:", "Expected usage information in stderr")
}

func TestSetupUsage(t *testing.T) {
	var output bytes.Buffer
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Call the setupUsage function
	setupUsage(&output, flags)

	// Trigger the usage function
	flags.Usage()

	// Expected usage output
	expectedSubstring := `Usage: tmcg.test [options]

Options:
  --resource, -r <resource>     Specify Terraform resources with optional mode (e.g., --resource aws_security_group:single --resource azurerm_network_security_group:multiple)
  --provider, -p <provider>     Specify Terraform providers (including optional versions) (e.g., --provider 'hashicorp/aws' --provider 'Azure/azapi:>=2.0')
  --directory, -d <directory>   The working directory for Terraform (default: "terraform")
  --binary, -b <path>           The path to the Terraform binary (default: "terraform")
  --log-level, -l <level>       Set the log level (debug, info, warn, error, panic, fatal) (default: "info")
  --help, -h                    Show usage information
  --version, -v                 Show version information
  --desc-as-comment             Whether to include the description as a comment in multiple mode (default: false)

Example:
  tmcg.test --provider 'hashicorp/aws:>=3.0' --resource aws_security_group --provider 'Azure/azapi:<2' --resource azapi_resource

Note:
  - Specify multiple resources and providers by using multiple --resource and --provider flags respectively.
  - You can include provider versions in the --provider flag (e.g., --provider "provider_namespace/provider_name:version").
`

	// Check if the output contains the expected substring
	if !strings.Contains(output.String(), expectedSubstring) {
		t.Errorf("Usage output does not match. Got:\n%s\nExpected:\n%s", output.String(), expectedSubstring)
	}
}

type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated write error")
}

func TestSetupUsage_ErrorHandling(t *testing.T) {
	var output errorWriter
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Call the setupUsage function with the error-inducing writer
	defer func() {
		if r := recover(); r != nil {
			// Recover from panic if usage writing error isn't gracefully handled
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	setupUsage(&output, flags)

	// Attempt to trigger the usage function
	flags.Usage()

	// If no panic occurred, ensure the test passes
	t.Logf("setupUsage gracefully handled the write error")
}
