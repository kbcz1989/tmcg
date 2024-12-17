package terraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tmcgParsing "tmcg/internal/tmcg/parsing"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

// TestCreateMainTF tests the createMainTF function for generating Terraform main files.
func TestCreateMainTF(t *testing.T) {
	testCases := []struct {
		name          string
		resources     []tmcgParsing.Resource
		cleanedSchema map[string]*tfjson.ProviderSchema
		expectedError bool
		expectedFile  string
		mockWriteFile bool
	}{
		{
			name: "Resource without underscore in name",
			resources: []tmcgParsing.Resource{
				{
					Name: "instance", // no underscore
					Mode: "single",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						Version:        ">= 3.0",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						// Match the exact resource name "instance"
						"instance": {
							Block: &tfjson.SchemaBlock{
								// No attributes needed, just ensure it's valid
							},
						},
					},
				},
			},
			expectedError: false,
			expectedFile:  "", // No file should be created
		},
		{
			name:      "No resources provided",
			resources: []tmcgParsing.Resource{}, // No resources
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{},
						},
					},
				},
			},
			expectedError: false,
			expectedFile:  "", // No file should be created
		},
		{
			name: "Missing provider schema",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{}, // Missing provider schema
			expectedError: false,
			expectedFile:  "", // No file should be created
		},
		{
			name: "Missing resource schema",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{}, // Missing resource schema
				},
			},
			expectedError: false,
			expectedFile:  "", // No file should be created
		},
		{
			name: "Invalid nested block",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"invalid_block": nil, // Invalid block
								},
							},
						},
					},
				},
			},
			expectedError: false,
			expectedFile:  "resource \"aws_instance\" \"this\" {\n}\n", // Minimal valid content
		},
		{
			name: "Single resource with attributes and nested blocks",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"name": {
										AttributeType: cty.String,
										Required:      true,
									},
									"tags": {
										AttributeType: cty.Map(cty.String),
										Optional:      true,
									},
								},
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"root_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										Block: &tfjson.SchemaBlock{
											NestedBlocks: map[string]*tfjson.SchemaBlockType{
												"child_block": {
													NestingMode: tfjson.SchemaNestingModeSingle,
													Block: &tfjson.SchemaBlock{
														Attributes: map[string]*tfjson.SchemaAttribute{
															"child_attr": {
																AttributeType: cty.String,
																Optional:      true,
															},
														},
													},
												},
											},
											Attributes: map[string]*tfjson.SchemaAttribute{
												"block_attribute": {
													AttributeType: cty.Number,
													Optional:      true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError: false,
			expectedFile: `resource "aws_instance" "this" {
  name = var.name

  dynamic "root_block" {
    for_each = can(coalesce(var.root_block)) ? flatten([var.root_block]) : []
    content {
      block_attribute = root_block.value.block_attribute

      dynamic "child_block" {
        for_each = can(coalesce(root_block.value.child_block)) ? flatten([root_block.value.child_block]) : []
        content {
          child_attr = child_block.value.child_attr
        }
      }
    }
  }

  tags = var.tags
}`,
		},
		{
			name: "Resource in multiple mode",
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						Version:        ">= 3.0",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"name": {
										AttributeType: cty.String,
										Required:      true,
									},
									"tags": {
										AttributeType: cty.Map(cty.String),
										Optional:      true,
									},
								},
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"root_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"block_attribute": {
													AttributeType: cty.Number,
													Optional:      true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError: false,
			expectedFile: `resource "aws_instance" "this" {
  for_each = { for i in coalesce(var.instances, []) : i.name => i }
  name     = each.value.name

  dynamic "root_block" {
    for_each = can(coalesce(each.value.root_block)) ? flatten([each.value.root_block]) : []
    content {
      block_attribute = root_block.value.block_attribute
    }
  }

  tags = each.value.tags
}`,
		},
		{
			name: "Invalid nested block",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"invalid_block": nil, // Invalid block
								},
							},
						},
					},
				},
			},
			expectedError: false,
			expectedFile:  "resource \"aws_instance\" \"this\" {\n}\n", // Minimal valid content
		},
		{
			name: "Error writing main.tf file",
			resources: []tmcgParsing.Resource{
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
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"name": {
										AttributeType: cty.String,
										Required:      true,
									},
								},
							},
						},
					},
				},
			},
			expectedError: true,
			mockWriteFile: true, // Simulate write failure
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary directory for the test
			dir := t.TempDir()

			// Optionally mock os.WriteFile for error simulation
			if tc.mockWriteFile {
				originalWriteFile := writeFile
				defer func() { writeFile = originalWriteFile }() // Restore original function

				// Mock writeFile to simulate an error
				writeFile = func(filename string, data []byte, perm os.FileMode) error {
					return fmt.Errorf("mock write error")
				}
			}

			// Call CreateMainTF with the cleaned schema and resources
			err := testTerraform.CreateMainTF(dir, tc.cleanedSchema, tc.resources)

			// Check for expected errors
			if tc.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to write main.tf")
				return
			}
			assert.NoError(t, err)

			// Initialize Terraform in the test directory
			tf, err := tfexec.NewTerraform(dir, "terraform")
			assert.NoError(t, err)

			// Format the generated file with terraform fmt
			if err := tf.FormatWrite(context.Background()); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format Terraform files: %v\n", err)
				os.Exit(1)
			}

			// Validate the output file if expected content is defined
			if tc.expectedFile != "" {
				filePath := filepath.Join(dir, "main.tf")
				content, err := os.ReadFile(filePath)
				assert.NoError(t, err)
				assert.Equal(t, strings.TrimSpace(tc.expectedFile), strings.TrimSpace(string(content)))
			}
		})
	}
}
