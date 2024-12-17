package parsing

import (
	"fmt"
	"os"
	"testing"
	"tmcg/internal/tmcg/logging"
)

func TestMain(m *testing.M) {
	// Initialize the global logger
	err := logging.InitLogger("debug") // Set the desired log level
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	os.Exit(m.Run())
}
