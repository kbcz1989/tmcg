package parsing

import (
	"testing"
	"tmcg/internal/tmcg/logging"

	"github.com/stretchr/testify/assert"
)

// TestParseResources tests the ParseResources function.
func TestParseResources(t *testing.T) {
	providers := map[string]Provider{
		"hashicorp/aws": {Namespace: "hashicorp", Name: "aws", Version: ">=3.0", NamespaceLower: "hashicorp", NameLower: "aws"},
		"azure/azapi":   {Namespace: "Azure", Name: "azapi", Version: ">=0", NamespaceLower: "azure", NameLower: "azapi"},
	}

	tests := []struct {
		name          string
		resourcePtrs  []string
		expected      []Resource
		expectError   bool
		errorContains string
	}{
		{
			name:         "Valid resources with single mode",
			resourcePtrs: []string{"aws_security_group:single"},
			expected: []Resource{
				{Name: "aws_security_group", Mode: "single", Provider: providers["hashicorp/aws"]},
			},
			expectError: false,
		},
		{
			name:         "Valid resources with default mode",
			resourcePtrs: []string{"aws_security_group", "azapi_resource"},
			expected: []Resource{
				{Name: "aws_security_group", Mode: "multiple", Provider: providers["hashicorp/aws"]},
				{Name: "azapi_resource", Mode: "multiple", Provider: providers["azure/azapi"]},
			},
			expectError: false,
		},
		{
			name:          "Invalid mode for resource",
			resourcePtrs:  []string{"aws_security_group:invalid"},
			expectError:   true,
			errorContains: "invalid mode",
		},
		{
			name:          "Resource without matching provider",
			resourcePtrs:  []string{"unknown_resource"},
			expectError:   true,
			errorContains: "no matching provider",
		},
		{
			name:          "Multiple single mode resources",
			resourcePtrs:  []string{"aws_security_group:single", "azapi_resource:single"},
			expectError:   true,
			errorContains: "only one resource of type 'single' is supported, due to potentially conflicting variable names",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := logging.GetGlobalLogger()
			parser := NewParser(logger)
			resources, err := parser.ParseResources(test.resourcePtrs, providers)
			if test.expectError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, resources)
			}
		})
	}
}
