// Package parsing contains functionality to parse Terraform resources,
// providers, and configuration data used by tmcg.
package parsing

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"tmcg/internal/tmcg/logging"
)

// Parser encapsulates parsing logic with logging
type Parser struct {
	logger logging.Logger
}

// NewParser creates a new Parser instance
func NewParser(logger logging.Logger) *Parser {
	return &Parser{logger: logger}
}

// Provider struct to hold provider information
type Provider struct {
	Namespace      string
	Name           string
	Version        string
	NamespaceLower string
	NameLower      string
}

// Resource struct to hold resource information with mode
type Resource struct {
	Name     string   // Resource name (e.g., "aws_vpc")
	Mode     string   // Mode: "single" or "multiple"
	Provider Provider // Associated Provider
}

// ParseProviderVersion parses the provider string to extract namespace, name, and optional version
func (p *Parser) ParseProviderVersion(provider string) (Provider, error) {
	// Regex to validate comma-separated version constraints
	versionRegex := regexp.MustCompile(`^((>=|<=|>|<|!=|~>)?\d+(\.\d+){0,2})(, ?(>=|<=|>|<|!=|~>)?\d+(\.\d+){0,2})*$`)

	// Split by colon to separate provider and optional version
	parts := strings.Split(provider, ":")
	if len(parts) == 0 || len(parts) > 2 {
		return Provider{}, fmt.Errorf("invalid provider format, expected 'namespace/name[:version]'")
	}

	// Split namespace and name
	nsAndNameParts := strings.Split(strings.TrimSpace(parts[0]), "/")
	if len(nsAndNameParts) != 2 || strings.TrimSpace(nsAndNameParts[0]) == "" || strings.TrimSpace(nsAndNameParts[1]) == "" {
		return Provider{}, fmt.Errorf("invalid provider format, expected 'namespace/name'")
	}

	// Extract version if provided, otherwise use default
	version := ">= 0"
	if len(parts) == 2 {
		version = strings.TrimSpace(parts[1])
		if version == "" || !versionRegex.MatchString(version) {
			return Provider{}, fmt.Errorf("invalid version format: '%s'", version)
		}
	}

	// Construct the provider
	return Provider{
		Namespace:      strings.TrimSpace(nsAndNameParts[0]),
		Name:           strings.TrimSpace(nsAndNameParts[1]),
		Version:        version,
		NamespaceLower: strings.ToLower(strings.TrimSpace(nsAndNameParts[0])),
		NameLower:      strings.ToLower(strings.TrimSpace(nsAndNameParts[1])),
	}, nil
}

// ParseProviders parses and validates provider strings into a map of Provider structs
func (p *Parser) ParseProviders(providerPtrs []string) (map[string]Provider, error) {
	providers := make(map[string]Provider)

	// Define a regex pattern for validating provider format
	providerRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+(:[a-zA-Z0-9.<>=~_-]+)?$`)

	for _, providerStr := range providerPtrs {
		// Validate the format using regex
		if !providerRegex.MatchString(providerStr) {
			return nil, fmt.Errorf("invalid provider format: '%s'. Expected format: 'namespace/name[:version]'", providerStr)
		}

		// Parse the provider version (existing logic)
		provider, err := p.ParseProviderVersion(providerStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing provider '%s': %w", providerStr, err)
		}

		// Generate a key for the provider (namespace/name)
		providerKey := fmt.Sprintf("%s/%s", provider.NamespaceLower, provider.NameLower)

		// Check for duplicate providers
		if _, exists := providers[providerKey]; exists {
			return nil, fmt.Errorf("duplicate provider found: %s", providerKey)
		}

		// Add the parsed provider to the map
		p.logger.Log("debug", "Parsed provider: %s", providerKey)
		providers[providerKey] = provider
	}

	return providers, nil
}

// ParseResources parses and validates resource strings into a slice of Resource structs
func (p *Parser) ParseResources(resourcePtrs []string, providers map[string]Provider) ([]Resource, error) {
	resources := []Resource{}
	singleModeCount := 0 // Counter for resources with "single" mode

	for _, resourceStr := range resourcePtrs {
		parts := strings.Split(resourceStr, ":")
		name := parts[0]
		mode := "multiple" // Default mode
		if len(parts) > 1 {
			mode = parts[1]
		}

		if mode != "single" && mode != "multiple" {
			return nil, fmt.Errorf("invalid mode for resource '%s': %s. Use 'single' or 'multiple'", name, mode)
		}

		if mode == "single" {
			singleModeCount++
			if singleModeCount > 1 {
				return nil, fmt.Errorf("only one resource of type 'single' is supported, due to potentially conflicting variable names")
			}
		}

		// Identify provider for the resource based on naming convention
		var associatedProvider Provider
		for _, provider := range providers {
			if strings.HasPrefix(name, provider.NameLower) {
				associatedProvider = provider
				break
			}
		}

		if (Provider{}) == associatedProvider {
			return nil, fmt.Errorf("no matching provider found for resource: %s", name)
		}

		resource := Resource{
			Name:     name,
			Mode:     mode,
			Provider: associatedProvider,
		}
		resources = append(resources, resource)

		p.logger.Log("debug", "Parsed resource: %s with mode: %s, associated provider: %+v", name, mode, associatedProvider)
	}

	return resources, nil
}

// ParseValidationErrorsFromJSON parses validation errors from terraform validate JSON output
func (p *Parser) ParseValidationErrorsFromJSON(jsonOutput string) (map[string][]string, error) {
	// Debug log: Indicate the start of JSON parsing
	p.logger.Log("debug", "Parsing validation errors from JSON output...")

	var parsedOutput struct {
		Diagnostics []struct {
			Severity string `json:"severity"`
			Address  string `json:"address"`
			Summary  string `json:"summary"`
			Detail   string `json:"detail"`
			Snippet  struct {
				Context string `json:"context"`
				Code    string `json:"code"`
			} `json:"snippet"`
		} `json:"diagnostics"`
	}

	// Attempt to unmarshal the JSON output
	if err := json.Unmarshal([]byte(jsonOutput), &parsedOutput); err != nil {
		p.logger.Log("error", "Failed to parse JSON output: %v", err)
		return nil, fmt.Errorf("failed to parse JSON output from terraform validate: %w", err)
	}

	// Log the number of diagnostics found
	p.logger.Log("debug", "Found %d diagnostics in JSON output.", len(parsedOutput.Diagnostics))

	invalidKeys := make(map[string][]string)

	// Process each diagnostic
	for _, diagnostic := range parsedOutput.Diagnostics {
		if diagnostic.Severity == "error" {
			// Extract the address, if available
			address := diagnostic.Address
			if address == "" && diagnostic.Snippet.Context != "" {
				// Attempt to extract resource name from context if address is empty
				context := diagnostic.Snippet.Context
				re := regexp.MustCompile(`resource\s+"([^"]+)"\s+"([^"]+)"`)
				matches := re.FindStringSubmatch(context)
				if len(matches) == 3 {
					address = fmt.Sprintf("%s.%s", matches[1], matches[2])
					p.logger.Log("debug", "Extracted address from context: %s", address)
				} else {
					p.logger.Log("warn", "Unable to extract address from context: %s", context)
					continue
				}
			}

			if address == "" {
				p.logger.Log("warn", "Skipping diagnostic with no address or context.")
				continue
			}

			p.logger.Log("debug", "Processing diagnostic for address: %s", address)

			// Attempt to extract invalid keys from the detail field
			detail := diagnostic.Detail
			if detail != "" {
				matches := regexp.MustCompile(`Can't configure a value for \"(.*?)\"`).FindStringSubmatch(detail)
				if len(matches) > 1 {
					attribute := matches[1]
					p.logger.Log("debug", "Found invalid attribute in detail: %s", attribute)
					invalidKeys[address] = append(invalidKeys[address], attribute)
					continue
				}
			}

			// Attempt to extract invalid keys from the summary field
			summary := diagnostic.Summary
			if summary != "" {
				matches := regexp.MustCompile(`\"(.*?)\": this field cannot be set`).FindStringSubmatch(summary)
				if len(matches) > 1 {
					attribute := matches[1]
					p.logger.Log("debug", "Found invalid attribute in summary: %s", attribute)
					invalidKeys[address] = append(invalidKeys[address], attribute)
					continue
				}

				matches = regexp.MustCompile(`invalid or unknown key: (\w+)`).FindStringSubmatch(summary)
				if len(matches) > 1 {
					attribute := matches[1]
					p.logger.Log("debug", "Found invalid attribute in summary (unknown key): %s", attribute)
					invalidKeys[address] = append(invalidKeys[address], attribute)
					continue
				}
			}

			// Fall back to extracting from the code snippet
			code := strings.TrimSpace(diagnostic.Snippet.Code)
			if code != "" {
				// Extract the attribute name before the '=' sign, if present
				if strings.Contains(code, "=") {
					attribute := strings.TrimSpace(strings.Split(code, "=")[0])
					if attribute != "" {
						p.logger.Log("debug", "Extracted invalid attribute from code snippet: %s", attribute)
						invalidKeys[address] = append(invalidKeys[address], attribute)
					}
				}
			}
		}
	}

	// Log the number of invalid attributes extracted
	p.logger.Log("info", "Extracted %d invalid attributes from diagnostics.", len(invalidKeys))

	return invalidKeys, nil
}
