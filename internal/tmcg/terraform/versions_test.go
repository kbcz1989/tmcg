package terraform

import (
	"os"
	"path/filepath"
	"testing"

	tmcgParsing "tmcg/internal/tmcg/parsing"

	"github.com/stretchr/testify/assert"
)

// TestCreateVersionsTF tests the CreateVersionsTF function for generating versions.tf.
func TestCreateVersionsTF(t *testing.T) {
	providers := map[string]tmcgParsing.Provider{
		"hashicorp/aws":    {Namespace: "hashicorp", Name: "aws", Version: ">= 3.0", NamespaceLower: "hashicorp", NameLower: "aws"},
		"hashicorp/random": {Namespace: "hashicorp", Name: "random", Version: ">= 2.0", NamespaceLower: "hashicorp", NameLower: "random"},
	}

	workingDir := t.TempDir()
	err := testTerraform.CreateVersionsTF(workingDir, providers)
	assert.NoError(t, err)

	filePath := filepath.Join(workingDir, "versions.tf")
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)

	expectedParts := []string{
		"terraform {",
		"required_providers {",
		"aws = {",
		"source  = \"hashicorp/aws\"",
		"version = \">= 3.0\"",
		"random = {",
		"source  = \"hashicorp/random\"",
		"version = \">= 2.0\"",
		"}",
		"}",
		"}",
	}

	for _, part := range expectedParts {
		assert.Contains(t, string(content), part, "Generated versions.tf is missing expected content")
	}
}
