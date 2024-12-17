package schema

import (
	"fmt"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	tmcgParsing "tmcg/internal/tmcg/parsing"
)

// MockLogger is a simple implementation of Logger for testing purposes
type MockLogger struct {
	Messages []string
}

// Log stores the formatted message in Messages
func (m *MockLogger) Log(level string, format string, args ...interface{}) {
	m.Messages = append(m.Messages, fmt.Sprintf(format, args...))
}

// TestFilterSchema tests the FilterSchema function
func TestFilterSchema(t *testing.T) {
	mockLogger := &MockLogger{}
	manager := NewSchemaManager(mockLogger)

	mockProviderSchemas := &tfjson.ProviderSchemas{
		FormatVersion: "0.1",
		Schemas: map[string]*tfjson.ProviderSchema{
			"hashicorp/aws": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"aws_instance": {
						Block: &tfjson.SchemaBlock{},
					},
					"aws_vpc": {
						Block: &tfjson.SchemaBlock{},
					},
				},
			},
		},
	}

	mockResources := []tmcgParsing.Resource{
		{Name: "aws_instance"},
	}

	expectedSchema := &tfjson.ProviderSchemas{
		FormatVersion: "0.1",
		Schemas: map[string]*tfjson.ProviderSchema{
			"hashicorp/aws": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"aws_instance": {
						Block: &tfjson.SchemaBlock{},
					},
				},
			},
		},
	}

	filteredSchema := manager.FilterSchema(mockProviderSchemas, mockResources)
	assert.Equal(t, expectedSchema, filteredSchema)
}

// TestRemoveComputedAttributes tests the RemoveComputedAttributes function
func TestRemoveComputedAttributes(t *testing.T) {
	mockLogger := &MockLogger{}
	manager := NewSchemaManager(mockLogger)

	mockProviderSchemas := &tfjson.ProviderSchemas{
		Schemas: map[string]*tfjson.ProviderSchema{
			"hashicorp/aws": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"aws_instance": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {
									Computed: true,
									Optional: false,
									Required: false, // Should be removed
								},
							},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"nested_block": {
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"nested_attr": {Computed: true, Optional: false, Required: false}, // Should be removed
											"valid_attr":  {Computed: true, Optional: true, Required: false},  // Should remain
										},
									},
								},
							},
						},
					},
					"aws_vpc": {
						Block: nil, // Nil block
					},
				},
			},
		},
	}

	expectedSchema := &tfjson.ProviderSchemas{
		Schemas: map[string]*tfjson.ProviderSchema{
			"hashicorp/aws": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"aws_instance": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{},
							NestedBlocks: map[string]*tfjson.SchemaBlockType{
								"nested_block": {
									Block: &tfjson.SchemaBlock{
										Attributes: map[string]*tfjson.SchemaAttribute{
											"valid_attr": {Computed: true, Optional: true, Required: false}, // Should remain
										},
									},
								},
							},
						},
					},
					"aws_vpc": {
						Block: nil, // Nil block should remain untouched
					},
				},
			},
		},
	}

	cleanedSchema := manager.RemoveComputedAttributes(mockProviderSchemas)
	assert.Equal(t, expectedSchema, cleanedSchema)
}

// TestRemoveInvalidAttributesFromSchema tests the RemoveInvalidAttributesFromSchema function
func TestRemoveInvalidAttributesFromSchema(t *testing.T) {
	mockLogger := &MockLogger{}
	manager := NewSchemaManager(mockLogger)

	mockProviderSchemas := map[string]*tfjson.ProviderSchema{
		"hashicorp/aws": {
			ResourceSchemas: map[string]*tfjson.Schema{
				"aws_instance": {
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"name": {
								AttributeType: cty.String,
							},
							"type": {
								AttributeType: cty.String,
							},
						},
					},
				},
				"aws_vpc": {
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"cidr_block": {
								AttributeType: cty.String,
							},
						},
					},
				},
			},
		},
	}

	mockValidationErrors := map[string][]string{
		"aws_instance": {"type", "nonexistent_attr"}, // One valid, one nonexistent attribute
		"aws_vpc":      {},                           // No invalid attributes for this resource
	}

	expectedSchema := &tfjson.ProviderSchemas{
		Schemas: map[string]*tfjson.ProviderSchema{
			"hashicorp/aws": {
				ResourceSchemas: map[string]*tfjson.Schema{
					"aws_instance": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"name": {
									AttributeType: cty.String,
								},
							},
						},
					},
					"aws_vpc": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"cidr_block": {
									AttributeType: cty.String,
								},
							},
						},
					},
				},
			},
		},
	}

	cleanedSchema := manager.RemoveInvalidAttributesFromSchema(mockProviderSchemas, mockValidationErrors)
	assert.Equal(t, expectedSchema, cleanedSchema)
}

func TestRemoveComputedAttributesFromBlock(t *testing.T) {
	tests := []struct {
		name          string
		inputBlock    *tfjson.SchemaBlock
		expectedBlock *tfjson.SchemaBlock
		expectedLogs  []string
	}{
		{
			name: "Remove computed-only attributes",
			inputBlock: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attribute_1": {Computed: true, Optional: false, Required: false}, // Computed-only
					"attribute_2": {Computed: true, Optional: true, Required: false},  // Not computed-only
					"attribute_3": {Computed: false, Optional: false, Required: true}, // Required
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"nested_block": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"nested_attr_1": {Computed: true, Optional: false, Required: false}, // Computed-only
								"nested_attr_2": {Computed: false, Optional: true, Required: false}, // Optional
							},
						},
					},
				},
			},
			expectedBlock: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attribute_2": {Computed: true, Optional: true, Required: false},  // Not computed-only
					"attribute_3": {Computed: false, Optional: false, Required: true}, // Required
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{
					"nested_block": {
						Block: &tfjson.SchemaBlock{
							Attributes: map[string]*tfjson.SchemaAttribute{
								"nested_attr_2": {Computed: false, Optional: true, Required: false}, // Optional
							},
						},
					},
				},
			},
			expectedLogs: []string{
				"Removed computed-only attribute: attribute_1",
				"Removed computed-only attribute: nested_attr_1",
			},
		},
		{
			name:          "Nil block",
			inputBlock:    nil,
			expectedBlock: nil,
			expectedLogs:  nil,
		},
		{
			name: "Block with no computed-only attributes",
			inputBlock: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attribute_1": {Computed: false, Optional: true, Required: false},
					"attribute_2": {Computed: true, Optional: true, Required: false},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{},
			},
			expectedBlock: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"attribute_1": {Computed: false, Optional: true, Required: false},
					"attribute_2": {Computed: true, Optional: true, Required: false},
				},
				NestedBlocks: map[string]*tfjson.SchemaBlockType{},
			},
			expectedLogs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock logger to capture logs
			mockLogger := &MockLogger{}

			// Create a SchemaManager with the mock logger
			sm := &SchemaManager{logger: mockLogger}

			// Run the function
			sm.RemoveComputedAttributesFromBlock(tt.inputBlock)

			// Assert the result
			assert.Equal(t, tt.expectedBlock, tt.inputBlock)

			// Assert the logs
			if tt.expectedLogs != nil {
				assert.ElementsMatch(t, mockLogger.Messages, tt.expectedLogs)
			} else {
				assert.Empty(t, mockLogger.Messages)
			}
		})
	}
}
