package terraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tmcgParsing "tmcg/internal/tmcg/parsing"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestCreateVariablesTF(t *testing.T) {
	tests := []struct {
		name              string
		dirFunc           func() string
		cleanedSchema     map[string]*tfjson.ProviderSchema
		resources         []tmcgParsing.Resource
		descAsComments    bool
		expectErr         bool
		expectedSubstring string // Part of the expected content or behavior we can assert on
	}{
		{
			name:           "no resources",
			dirFunc:        t.TempDir,
			cleanedSchema:  map[string]*tfjson.ProviderSchema{},
			resources:      []tmcgParsing.Resource{},
			descAsComments: false,
			expectErr:      false,
			// No variables.tf should be created.
			expectedSubstring: "",
		},
		{
			name:    "provider not found",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/random": {}, // different provider
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments: false,
			expectErr:      false,
			// In this scenario, variables.tf should be empty or not have variables for aws_instance
			expectedSubstring: "",
		},
		{
			name:    "resource schema not found",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						// no aws_instance schema provided
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    false,
			expectErr:         false,
			expectedSubstring: "",
		},
		{
			name:    "multiple mode resource with optional attribute",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"ami": {
										AttributeType: cty.String,
										Required:      true,
									},
									"instance_type": {
										AttributeType: cty.String,
										Optional:      true,
										Description:   "The type of instance",
									},
								},
							},
						},
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    true,
			expectErr:         false,
			expectedSubstring: "variable \"instances\"",
		},
		{
			name:    "single mode resource with required and optional attributes",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_s3_bucket": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"bucket": {
										AttributeType: cty.String,
										Required:      true,
										Description:   "The name of the bucket",
									},
									"acl": {
										AttributeType: cty.String,
										Optional:      true,
										Description:   "The ACL of the bucket",
									},
									"tags": {
										AttributeType: cty.Map(cty.String),
										Optional:      true,
										Description:   "A map of tags to assign to the bucket",
									},
									"cors_rule": {
										AttributeType: cty.Object(map[string]cty.Type{
											"allowed_methods": cty.List(cty.String),
											"allowed_origins": cty.List(cty.String),
										}),
										Optional:    true,
										Description: "A single CORS rule object with allowed methods and origins",
									},
									"lifecycle_rules": {
										AttributeType: cty.List(cty.Object(map[string]cty.Type{
											"id":     cty.String,
											"status": cty.String,
										})),
										Optional:    true,
										Description: "A list of lifecycle rule objects, each with an id and status",
									},
									"set_of_strings": {
										AttributeType: cty.Set(cty.String),
										Optional:      true,
										Description:   "A set of strings",
									},
								},
							},
						},
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_s3_bucket",
					Mode: "single",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    false,
			expectErr:         false,
			expectedSubstring: "variable \"bucket\"",
		},
		{
			name:    "resource with nested blocks",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"root_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										MinItems:    0,
										MaxItems:    1,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"name": {
													AttributeType: cty.String,
													Optional:      true,
												},
											},
										},
									},
									"multiple_block": {
										NestingMode: tfjson.SchemaNestingModeList,
										MinItems:    0,
										MaxItems:    0,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"value": {
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
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "single",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    false,
			expectErr:         false,
			expectedSubstring: "variable \"root_block\"",
		},
		{
			name:    "description as comments flag",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"ami": {
										AttributeType: cty.String,
										Required:      true,
										Description:   "This is the AMI attribute",
									},
									"unknown_attribute": {
										Required: true,
									},
								},
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"root_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										MinItems:    0,
										MaxItems:    1,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"name": {
													AttributeType: cty.String,
													Optional:      true,
												},
											},
										},
									},
									"multiple_block": {
										NestingMode: tfjson.SchemaNestingModeList,
										MinItems:    0,
										MaxItems:    0,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"value": {
													AttributeType: cty.Number,
													Optional:      true,
												},
											},
											Description: "This is the multiple block",
										},
									},
									"multiple_block_set_required": {
										NestingMode: tfjson.SchemaNestingModeSet,
										MinItems:    1,
										MaxItems:    0,
										Block: &tfjson.SchemaBlock{
											Attributes: map[string]*tfjson.SchemaAttribute{
												"value": {
													AttributeType: cty.Number,
													Optional:      true,
												},
											},
											Description: "This is the multiple block",
										},
									},
									"unknown_nesting_mode": {
										MinItems: 1,
										MaxItems: 0,
									},
								},
							},
						},
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    true,
			expectErr:         false,
			expectedSubstring: "// This is the AMI attribute",
		},
		{
			name: "write file fails",
			dirFunc: func() string {
				// Create a directory and ensure it is write-protected
				dir := t.TempDir()
				err := os.Chmod(dir, 0555)
				if err != nil {
					t.Fatalf("Failed to set write-protection on directory: %v", err)
				}

				// Verify the directory is write-protected
				testFile := filepath.Join(dir, "test.txt")
				err = os.WriteFile(testFile, []byte("test"), 0644)
				if err == nil {
					t.Fatalf("Expected write-protected directory, but was able to write: %v", testFile)
				}

				return dir
			},
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"ami": {
										AttributeType: cty.String,
										Required:      true,
									},
								},
							},
						},
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "multiple",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments: false,
			expectErr:      true,
		},
		{
			name:    "Edge cases with nil attributes and blocks",
			dirFunc: t.TempDir,
			cleanedSchema: map[string]*tfjson.ProviderSchema{
				"registry.terraform.io/hashicorp/aws": {
					ResourceSchemas: map[string]*tfjson.Schema{
						"aws_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"nil_attr": nil, // Triggers attrSchema == nil condition
								},
								NestedBlocks: map[string]*tfjson.SchemaBlockType{
									"nil_block": nil, // Triggers block == nil condition
									"no_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										Block:       nil, // block.Block == nil because block is nil
									},
									"empty_block": {
										NestingMode: tfjson.SchemaNestingModeSingle,
										Block: &tfjson.SchemaBlock{
											Attributes:   map[string]*tfjson.SchemaAttribute{},
											NestedBlocks: map[string]*tfjson.SchemaBlockType{},
											// Triggers condition where both attributes and nested blocks are empty
										},
									},
								},
							},
						},
					},
				},
			},
			resources: []tmcgParsing.Resource{
				{
					Name: "aws_instance",
					Mode: "single",
					Provider: tmcgParsing.Provider{
						Namespace:      "hashicorp",
						Name:           "aws",
						NamespaceLower: "hashicorp",
						NameLower:      "aws",
					},
				},
			},
			descAsComments:    false,
			expectErr:         false,
			expectedSubstring: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.dirFunc()
			err := testTerraform.CreateVariablesTF(dir, tt.cleanedSchema, tt.resources, tt.descAsComments)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Initialize Terraform in the test directory
			tf, err := tfexec.NewTerraform(dir, "terraform")
			assert.NoError(t, err)

			// Format the generated file with terraform fmt
			if err := tf.FormatWrite(context.Background()); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format Terraform files: %v\n", err)
				os.Exit(1)
			}

			// If we expect some substring in the variables.tf, check it
			if tt.expectedSubstring != "" && !tt.expectErr {
				content, readErr := os.ReadFile(filepath.Join(dir, "variables.tf"))
				require.NoError(t, readErr)
				assert.Contains(t, string(content), tt.expectedSubstring)
			}

			// If no resources or provider not found, variables.tf might not exist
			if len(tt.resources) == 0 || (len(tt.cleanedSchema) == 0 && !tt.expectErr) {
				_, statErr := os.Stat(filepath.Join(dir, "variables.tf"))
				assert.True(t, os.IsNotExist(statErr))
			}
		})
	}
}
