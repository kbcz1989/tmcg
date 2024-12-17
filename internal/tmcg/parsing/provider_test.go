package parsing

import (
	"testing"

	"tmcg/internal/tmcg/logging"

	"github.com/stretchr/testify/assert"
)

// TestParseProviderVersion tests ParseProviderVersion for correct parsing of provider strings.
func TestParseProviderVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Provider
		expectError bool
	}{
		{"Valid provider with version", "hashicorp/aws:>=3.0", Provider{Namespace: "hashicorp", Name: "aws", Version: ">=3.0", NamespaceLower: "hashicorp", NameLower: "aws"}, false},
		{"Valid provider without version", "Azure/azapi", Provider{Namespace: "Azure", Name: "azapi", Version: ">= 0", NamespaceLower: "azure", NameLower: "azapi"}, false},
		{"Invalid format without slash", "invalidprovider", Provider{}, true},
		{"Invalid format with only namespace", "namespace/", Provider{}, true},
		{"Invalid format with too many elements", "namespace/name:1.0:wrong", Provider{}, true},
		{"Empty string input", "", Provider{}, true},
		{"Leading and trailing whitespace", "  hashicorp/aws : >=3.0  ", Provider{Namespace: "hashicorp", Name: "aws", Version: ">=3.0", NamespaceLower: "hashicorp", NameLower: "aws"}, false},
		{"Empty version after colon", "hashicorp/aws:", Provider{}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := logging.GetGlobalLogger()
			parser := NewParser(logger)
			provider, err := parser.ParseProviderVersion(test.input)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, provider)
			}
		})
	}
}

// TestParseProviders tests ParseProviders for unique provider parsing and error handling.
func TestParseProviders(t *testing.T) {
	tests := []struct {
		name          string
		providerPtrs  []string
		expected      map[string]Provider
		expectError   bool
		errorContains string
	}{
		{"Valid providers", []string{"hashicorp/aws:>=3.0", "Azure/azapi"}, map[string]Provider{
			"hashicorp/aws": {Namespace: "hashicorp", Name: "aws", Version: ">=3.0", NamespaceLower: "hashicorp", NameLower: "aws"},
			"azure/azapi":   {Namespace: "Azure", Name: "azapi", Version: ">= 0", NamespaceLower: "azure", NameLower: "azapi"},
		}, false, ""},
		{"Duplicate providers", []string{"hashicorp/aws:>=3.0", "hashicorp/aws"}, nil, true, "duplicate provider found"},
		{"Invalid provider format", []string{"invalidprovider"}, nil, true, "invalid provider format"},
		{"Empty input list", []string{}, map[string]Provider{}, false, ""},
		{"Valid regex but invalid version", []string{"hashicorp/aws:invalid-version"}, nil, true, "error parsing provider"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := logging.GetGlobalLogger()
			parser := NewParser(logger)
			providers, err := parser.ParseProviders(test.providerPtrs)
			if test.expectError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, providers)
			}
		})
	}
}
