package main

import (
	"context"
	"os"
	"testing"

	tmcgLogging "tmcg/internal/tmcg/logging"
	tmcgParsing "tmcg/internal/tmcg/parsing"
	tmcgSchema "tmcg/internal/tmcg/schema"
	tmcgTerraform "tmcg/internal/tmcg/terraform"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/stretchr/testify/assert"
)

func TestEndToEndTerraformWorkflow(t *testing.T) {
	// Setup working directory
	dir := t.TempDir()

	// Initialize the logger
	err := tmcgLogging.InitLogger("info")
	assert.NoError(t, err)
	logger := tmcgLogging.GetGlobalLogger()

	// Initialize schema manager and Terraform instance
	schemaManager := tmcgSchema.NewSchemaManager(logger)
	terraform := tmcgTerraform.NewTf(logger)

	// Step 1: Create versions.tf
	providers := map[string]tmcgParsing.Provider{
		"hashicorp/aws": {
			Namespace:      "hashicorp",
			Name:           "aws",
			Version:        ">= 3.0",
			NamespaceLower: "hashicorp",
			NameLower:      "aws",
		},
		"hashicorp/random": {
			Namespace:      "hashicorp",
			Name:           "random",
			Version:        ">= 2.0",
			NamespaceLower: "hashicorp",
			NameLower:      "random",
		},
	}

	err = terraform.CreateVersionsTF(dir, providers)
	assert.NoError(t, err)

	// Step 2: Initialize Terraform
	tf, err := tfexec.NewTerraform(dir, "terraform")
	assert.NoError(t, err)

	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	assert.NoError(t, err)

	// Step 3: Fetch provider schema
	schemaJSON, err := tf.ProvidersSchema(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, schemaJSON)

	// Step 4: Define resources and filter the provider schema for required resources
	resources := []tmcgParsing.Resource{
		{
			Name: "aws_instance",
			Mode: "single",
			Provider: tmcgParsing.Provider{
				Namespace:      "hashicorp",
				Name:           "aws",
				Version:        ">= 3.0",
				NamespaceLower: "hashicorp",
				NameLower:      "aws",
			},
		},
	}

	filteredSchema := schemaManager.FilterSchema(schemaJSON, resources)
	assert.NotNil(t, filteredSchema)

	// Step 5: Remove computed-only attributes from the filtered schema
	cleanedSchema := schemaManager.RemoveComputedAttributes(filteredSchema)
	assert.NotNil(t, cleanedSchema)

	// Step 6: Create main.tf
	err = terraform.CreateMainTF(dir, cleanedSchema.Schemas, resources)
	assert.NoError(t, err)

	// Step 7: Create variables.tf
	err = terraform.CreateVariablesTF(dir, cleanedSchema.Schemas, resources, false)
	assert.NoError(t, err)

	// Step 8: Run terraform validate
	validationErrors, err := terraform.RunTerraformValidate(tf)
	assert.NoError(t, err)
	if len(validationErrors) > 0 {
		// Remove invalid attributes and regenerate files if there are validation errors
		cleanedSchema = schemaManager.RemoveInvalidAttributesFromSchema(cleanedSchema.Schemas, validationErrors)
		assert.NotNil(t, cleanedSchema)

		// Regenerate main.tf
		err = terraform.CreateMainTF(dir, cleanedSchema.Schemas, resources)
		assert.NoError(t, err)

		// Regenerate variables.tf
		err = terraform.CreateVariablesTF(dir, cleanedSchema.Schemas, resources, false)
		assert.NoError(t, err)

		// Run terraform validate again to ensure it passes after cleanup
		validationErrors, err = terraform.RunTerraformValidate(tf)
		assert.NoError(t, err)
		assert.Empty(t, validationErrors)
	}

	// Step 9: Run terraform fmt
	err = terraform.RunTerraformFmt(tf.WorkingDir(), tf.FormatWrite)
	assert.NoError(t, err)

	// Optional: Clean up
	assert.NoError(t, os.RemoveAll(dir))
}
