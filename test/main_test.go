package main

import (
	"fmt"
	"os"
	"testing"

	"tmcg/internal/tmcg/logging"
)

// TestMain is the entry point for running all tests in this package.
func TestMain(m *testing.M) {
	// Initialize the logger with a default level
	err := logging.InitLogger("info")
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Run tests and exit with the appropriate status
	os.Exit(m.Run())
}

