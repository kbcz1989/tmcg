package parsing

import (
	"testing"
	"tmcg/internal/tmcg/logging"

	"github.com/stretchr/testify/assert"
)

func TestParseValidationErrorsFromJSON(t *testing.T) {
	tests := []struct {
		name          string
		inputJSON     string
		expectedKeys  map[string][]string
		expectedError bool
	}{
		{
			name: "Valid diagnostics with all fields",
			inputJSON: `{
				"diagnostics": [
					{
						"severity": "error",
						"address": "aws_instance.example",
						"summary": "\"key_name\": this field cannot be set",
						"detail": "",
						"snippet": {
							"context": "",
							"code": ""
						}
					},
					{
						"severity": "error",
						"address": "",
						"summary": "",
						"detail": "Can't configure a value for \"user_data\"",
						"snippet": {
							"context": "resource \"aws_instance\" \"example\"",
							"code": "user_data = <<EOF\n# User data script\nEOF"
						}
					},
					{
						"severity": "error",
						"address": "azurerm_kubernetes_cluster.example",
						"summary": "Invalid or unknown key",
						"detail": "",
						"snippet": {
							"context": "resource \"azurerm_kubernetes_cluster\" \"example\"",
							"code": "  id       = \"something\""
						}
					},
					{
						"severity": "error",
						"address": "panos_security_policy.example",
						"summary": "invalid or unknown key: id",
						"detail": "",
						"snippet": {
							"context": "resource \"panos_security_policy\" \"example\" {",
							"code": ""
						}
					}
				]
			}`,
			expectedKeys: map[string][]string{
				"aws_instance.example":               {"key_name", "user_data"},
				"azurerm_kubernetes_cluster.example": {"id"},
				"panos_security_policy.example":      {"id"},
			},
			expectedError: false,
		},
		{
			name: "Empty diagnostics",
			inputJSON: `{
				"diagnostics": []
			}`,
			expectedKeys:  map[string][]string{},
			expectedError: false,
		},
		{
			name:          "Invalid JSON",
			inputJSON:     `INVALID_JSON`,
			expectedKeys:  nil,
			expectedError: true,
		},
		{
			name: "Diagnostics with no address or context",
			inputJSON: `{
				"diagnostics": [
					{
						"severity": "error",
						"address": "",
						"summary": "Invalid attribute",
						"detail": "",
						"snippet": {
							"context": "",
							"code": ""
						}
					}
				]
			}`,
			expectedKeys:  map[string][]string{},
			expectedError: false,
		},
		{
			name: "Extract attribute from snippet",
			inputJSON: `{
				"diagnostics": [
					{
						"severity": "error",
						"address": "aws_instance.example",
						"summary": "",
						"detail": "",
						"snippet": {
							"context": "",
							"code": "invalid_attribute = \"value\""
						}
					}
				]
			}`,
			expectedKeys: map[string][]string{
				"aws_instance.example": {"invalid_attribute"},
			},
			expectedError: false,
		},
		{
			name: "Diagnostics with invalid context format",
			inputJSON: `{
				"diagnostics": [
					{
						"severity": "error",
						"address": "",
						"summary": "Invalid context format",
						"detail": "",
						"snippet": {
							"context": "invalid context without resource definition",
							"code": ""
						}
					}
				]
			}`,
			expectedKeys:  map[string][]string{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.GetGlobalLogger()
			parser := NewParser(logger)
			actualKeys, err := parser.ParseValidationErrorsFromJSON(tt.inputJSON)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKeys, actualKeys)
			}
		})
	}
}
