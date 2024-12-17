package terraform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"tmcg/internal/tmcg/logging"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/stretchr/testify/assert"
)

var testTerraform *Tf

func TestMain(m *testing.M) {
	// Initialize the global logger
	err := logging.InitLogger("debug") // Set the desired log level
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger := logging.GetGlobalLogger()
	testTerraform = NewTf(logger)

	if testTerraform == nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Terraform instance.\n")
		os.Exit(1)
	}

	// Run tests
	os.Exit(m.Run())
}

func TestRunTerraformValidate(t *testing.T) {
	// Ensure the proper Terraform binary is available
	tf, err := tfexec.NewTerraform(t.TempDir(), "terraform")
	assert.NoError(t, err)

	// Run the function
	validationErrors, err := testTerraform.RunTerraformValidate(tf)
	assert.NoError(t, err)
	assert.Empty(t, validationErrors, "Expected no validation errors")
}

func TestRunTerraformFmt(t *testing.T) {
	mockSuccess := func(ctx context.Context, opts ...tfexec.FormatOption) error {
		return nil
	}

	mockFailure := func(ctx context.Context, opts ...tfexec.FormatOption) error {
		return fmt.Errorf("mock fmt failure")
	}

	t.Run("Success", func(t *testing.T) {
		err := testTerraform.RunTerraformFmt("/fake/dir", mockSuccess)
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		err := testTerraform.RunTerraformFmt("/fake/dir", mockFailure)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock fmt failure")
	})
}

func TestCleanupHCLFile(t *testing.T) {
	// Create a mock HCL file
	file := hclwrite.NewEmptyFile()
	file.Body().AppendUnstructuredTokens(hclwrite.TokensForIdentifier(`
resource "aws_instance" "example" {
  ami           = "ami-123456"
  instance_type = "t2.micro"


  dynamic "ebs_block_device" {
    for_each = var.ebs_block_device


	content {
	  device_name = ebs_block_device.value.device_name
    }


  }


}


`))

	// Call the function
	testTerraform.cleanupHCLFile(file)

	// Validate the result
	expected := `
resource "aws_instance" "example" {
  ami           = "ami-123456"
  instance_type = "t2.micro"

  dynamic "ebs_block_device" {
    for_each = var.ebs_block_device

	content {
	  device_name = ebs_block_device.value.device_name
    }
  }
}
`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(file.Bytes())))
}

func TestValidateTerraformBinary(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	t.Run("ValidBinary", func(t *testing.T) {
		// Mock exec.LookPath to return a valid path
		lookPath = func(binary string) (string, error) {
			return "/usr/local/bin/terraform", nil
		}

		result, err := testTerraform.ValidateTerraformBinary("terraform")
		assert.NoError(t, err)
		assert.Equal(t, "/usr/local/bin/terraform", result)
	})

	t.Run("InvalidBinary", func(t *testing.T) {
		// Mock exec.LookPath to return an error
		lookPath = func(binary string) (string, error) {
			return "", errors.New("not found")
		}

		result, err := testTerraform.ValidateTerraformBinary("invalid-binary")
		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "terraform binary not found")
	})
}
