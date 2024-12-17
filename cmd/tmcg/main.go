// Package main is the entry point for the tmcg application. It handles command-line
// arguments and orchestrates the functionality of tmcg.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tmcg/internal/tmcg/logging"
	tmcgParsing "tmcg/internal/tmcg/parsing"
	tmcgSchema "tmcg/internal/tmcg/schema"
	tmcgTerraform "tmcg/internal/tmcg/terraform"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/spf13/pflag"
)

// Custom flag to handle a slice of strings for resources and providers
type stringSliceFlag []string

// Set parses and sets the value
func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// String returns a string representation of the flag value
func (f *stringSliceFlag) String() string {
	return strings.Join(*f, ",")
}

// Type returns a string to describe the type of the flag
func (f *stringSliceFlag) Type() string {
	return "stringSlice"
}

var (
	resourcePtrs       stringSliceFlag
	providerPtrs       stringSliceFlag
	workingDir         string
	binaryPath         string
	logLevel           string
	helpFlag           bool
	versionFlag        bool
	descAsCommentsFlag bool
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	// Initialize the logger
	if err := logging.InitLogger("info"); err != nil {
		fmt.Println("Failed to initialize logger:", err)
		os.Exit(1)
	}
	logger := logging.GetGlobalLogger()

	Setup(os.Args, os.Stdout, os.Stderr, os.Exit, logger)
}

func Setup(args []string, stdout, stderr io.Writer, exitFunc func(int), logger logging.Logger) {
	// Create a new FlagSet for this run
	flags := pflag.NewFlagSet("tmcg", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	// Define command-line flags
	flags.VarP(&resourcePtrs, "resource", "r", "Specify Terraform resources with optional mode (e.g., --resource aws_security_group:single --resource azurerm_network_security_group:multiple)")
	flags.VarP(&providerPtrs, "provider", "p", "Specify Terraform providers (including optional versions) using multiple --provider flags (e.g., --provider 'hashicorp/aws' --provider 'Azure/azapi:>=2.0')")
	flags.StringVarP(&workingDir, "directory", "d", "terraform", "The working directory for Terraform")
	flags.StringVarP(&binaryPath, "binary", "b", "terraform", "The path to the Terraform binary")
	flags.StringVarP(&logLevel, "log-level", "l", "info", "Set the log level")
	flags.BoolVarP(&helpFlag, "help", "h", false, "Show usage information")
	flags.BoolVarP(&versionFlag, "version", "v", false, "Show version information")
	flags.BoolVar(&descAsCommentsFlag, "desc-as-comment", false, "Include description as a comment")

	// Update the Usage handler
	setupUsage(stdout, flags)

	// Parse flags
	if err := flags.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error parsing flags: %v\n", err)
		exitFunc(1)
		return
	}

	// Handle --version flag
	if versionFlag {
		_, _ = fmt.Fprintf(stdout, "tmcg version: %s\nCommit: %s\nBuilt on: %s\n", version, commit, buildDate)
		exitFunc(0)
		return
	}

	// Handle --help flag
	if helpFlag {
		flags.Usage()
		exitFunc(0)
		return
	}

	// Validate inputs
	if len(resourcePtrs) == 0 || len(providerPtrs) == 0 {
		logger.Log("error", "Missing required arguments: resources or providers")
		flags.Usage()
		exitFunc(1)
		return
	}

	// Execute the main pipeline
	Run(os.Exit, logger)
}

func Run(exitFunc func(int), logger logging.Logger) {
	logger.Log("info", "Validating provided providers and resources...")

	// Parse and validate providers
	parser := tmcgParsing.NewParser(logger)
	providers, err := parser.ParseProviders(providerPtrs)
	if err != nil {
		logger.Log("error", "Failed to parse providers from provided pointers: %v", err)
		pflag.Usage()
		exitFunc(1)
	}

	for _, provider := range providers {
		logger.Log("debug", "Parsed provider: %+v", provider)
	}

	// Parse and validate resources
	resources, err := parser.ParseResources(resourcePtrs, providers)
	if err != nil {
		logger.Log("error", "Failed to parse resources from provided pointers and providers: %v", err)
		pflag.Usage()
		exitFunc(1)
	}

	for _, resource := range resources {
		logger.Log("debug", "Parsed resource: %+v", resource)
	}

	// Ensure the working directory exists
	err = os.MkdirAll(workingDir, 0755)
	if err != nil {
		logger.Log("error", "Error creating working directory: %s", err)
		exitFunc(1)
	}
	logger.Log("info", "Working directory set to: %s", workingDir)

	// Validate Terraform binary
	logger.Log("debug", "Using Terraform binary: %s", binaryPath)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		logger.Log("error", "Terraform binary not found at: %s", binaryPath)
		exitFunc(1)
	}

	// Start timer for execution
	startTime := time.Now()

	defer func() {
		logger.Log("info", "Execution completed in %s", time.Since(startTime))
	}()

	// Step 1: Initialize Terraform
	logger.Log("info", "Initializing Terraform in directory: %s", workingDir)
	tf, err := tfexec.NewTerraform(workingDir, binaryPath)
	if err != nil {
		logger.Log("error", "Error initializing Terraform: %s", err)
		exitFunc(1)
	}

	// Step 2: Create versions.tf
	logger.Log("info", "Creating versions.tf with provider definitions...")
	terraform := tmcgTerraform.NewTf(logger)
	err = terraform.CreateVersionsTF(workingDir, providers)
	if err != nil {
		logger.Log("error", "Error creating versions.tf: %s", err)
		exitFunc(1)
	}

	// Step 3: Run terraform init
	logger.Log("info", "Running terraform init...")
	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		logger.Log("error", "Error running terraform init: %s", err)
		exitFunc(1)
	}

	// Step 4: Fetch provider schema
	logger.Log("info", "Fetching provider schema...")
	schemaJSON, err := tf.ProvidersSchema(context.Background())
	if err != nil {
		logger.Log("error", "Error fetching provider schema: %s", err)
		exitFunc(1)
	}
	logger.Log("debug", "Fetched provider schema: %+v", schemaJSON)

	// Step 5: Filter the provider schema for required resources
	logger.Log("info", "Filtering the provider schema for required resources...")
	err = logging.InitLogger("info")
	if err != nil {
		fmt.Println("Failed to initialize logger:", err)
		os.Exit(1)
	}
	schemaManager := tmcgSchema.NewSchemaManager(logging.GetGlobalLogger())
	filteredSchema := schemaManager.FilterSchema(schemaJSON, resources)
	logger.Log("debug", "Filtered provider schema: %+v", filteredSchema)

	// Step 6: Remove computed-only attributes from the filtered schema
	logger.Log("info", "Removing computed-only attributes from the filtered schema...")
	cleanedSchema := schemaManager.RemoveComputedAttributes(filteredSchema)
	logger.Log("debug", "Cleaned provider schema: %+v", cleanedSchema)

	// // Step 7: Generate main.tf
	logger.Log("info", "Generating main.tf...")
	err = terraform.CreateMainTF(workingDir, cleanedSchema.Schemas, resources)
	if err != nil {
		logger.Log("error", "Error creating main.tf: %s", err)
		exitFunc(1)
	}

	// Step 8: Generate variables.tf
	logger.Log("info", "Generating variables.tf...")
	err = terraform.CreateVariablesTF(workingDir, cleanedSchema.Schemas, resources, descAsCommentsFlag)
	if err != nil {
		logger.Log("error", "Error creating variables.tf: %s", err)
		exitFunc(1)
	}

	// Step 9: Run terraform validate
	logger.Log("info", "Running terraform validate...")
	validationErrors, err := terraform.RunTerraformValidate(tf)
	if err != nil {
		logger.Log("error", "Error running terraform validate: %s", err)
		exitFunc(1)
	}
	logger.Log("debug", "Validation output: %+v", validationErrors)

	// Step 10: Remove invalid attributes from the cleaned schema
	if len(validationErrors) > 0 {
		logger.Log("info", "Removing invalid attributes from the cleaned schema...")
		cleanedSchema = schemaManager.RemoveInvalidAttributesFromSchema(cleanedSchema.Schemas, validationErrors)
		logger.Log("info", "Invalid attributes removed. Regenerating main.tf and variables.tf...")

		// Regenerate main.tf
		err = terraform.CreateMainTF(workingDir, cleanedSchema.Schemas, resources)
		if err != nil {
			logger.Log("error", "Error creating main.tf after cleaning schema: %s", err)
			exitFunc(1)
		}

		// Regenerate variables.tf
		err = terraform.CreateVariablesTF(workingDir, cleanedSchema.Schemas, resources, descAsCommentsFlag)
		if err != nil {
			logger.Log("error", "Error creating variables.tf after cleaning schema: %s", err)
			exitFunc(1)
		}
	} else {
		logger.Log("info", "No invalid attributes found, no need to modify the schema.")
	}

	// Step 11: Run final terraform validate
	logger.Log("info", "Running terraform validate...")
	validationErrors, err = terraform.RunTerraformValidate(tf)
	if err != nil {
		logger.Log("error", "Error running terraform validate: %s", err)
		exitFunc(1)
	}

	// Check and log validation errors
	if len(validationErrors) == 0 {
		logger.Log("info", "Validation completed successfully with no errors.")
	} else {
		logger.Log("info", "Validation detected the following issues:")
		for file, errors := range validationErrors {
			logger.Log("info", "File: %s", file)
			for _, issue := range errors {
				logger.Log("info", "  - %s", issue)
			}
		}
	}

	// Step 12: Run terraform fmt
	logger.Log("info", "Running terraform fmt on directory: %s", workingDir)
	err = terraform.RunTerraformFmt(tf.WorkingDir(), tf.FormatWrite)
	if err != nil {
		logger.Log("error", "Error running terraform fmt: %v", err)
		exitFunc(1)
	}
	logger.Log("info", "Process completed successfully.")
}

// Set a custom usage message
func setupUsage(output io.Writer, flags *pflag.FlagSet) {
	// Get the base name of the program
	programName := filepath.Base(os.Args[0])

	flags.Usage = func() {
		if _, err := fmt.Fprintf(output, `Usage: %s [options]

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
  %s --provider 'hashicorp/aws:>=3.0' --resource aws_security_group --provider 'Azure/azapi:<2' --resource azapi_resource

Note:
  - Specify multiple resources and providers by using multiple --resource and --provider flags respectively.
  - You can include provider versions in the --provider flag (e.g., --provider "provider_namespace/provider_name:version").
`, programName, programName); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing usage information: %v\n", err)
		}

	}
}
