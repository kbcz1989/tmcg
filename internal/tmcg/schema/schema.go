// Package schema manages Terraform provider schemas and resource attributes,
// facilitating the generation of Terraform configurations.
package schema

import (
	"strings"

	"tmcg/internal/tmcg/logging"
	"tmcg/internal/tmcg/parsing"

	tfjson "github.com/hashicorp/terraform-json"
)

// SchemaManager is responsible for managing and filtering schemas.
type SchemaManager struct {
	logger logging.Logger
}

// NewSchemaManager creates a new instance of SchemaManager.
func NewSchemaManager(logger logging.Logger) *SchemaManager {
	return &SchemaManager{logger: logger}
}

// FilterSchema filters the fetched JSON schema for only the required resources.
func (sm *SchemaManager) FilterSchema(providerSchemas *tfjson.ProviderSchemas, resources []parsing.Resource) *tfjson.ProviderSchemas {
	sm.logger.Log("info", "Starting to filter provider schemas for required resources...")

	filteredProviderSchemas := &tfjson.ProviderSchemas{
		FormatVersion: providerSchemas.FormatVersion,
		Schemas:       make(map[string]*tfjson.ProviderSchema),
	}

	// Create a set of required resources for quick lookup.
	requiredResources := make(map[string]bool)
	for _, resource := range resources {
		requiredResources[resource.Name] = true
	}

	// Iterate over the provider schemas to filter only those required resources.
	for providerKey, providerSchema := range providerSchemas.Schemas {
		// Initialize a new ProviderSchema to hold filtered resources.
		filteredProviderSchema := &tfjson.ProviderSchema{
			ResourceSchemas: make(map[string]*tfjson.Schema),
		}

		for resourceName, resourceSchema := range providerSchema.ResourceSchemas {
			if _, exists := requiredResources[resourceName]; exists {
				filteredProviderSchema.ResourceSchemas[resourceName] = resourceSchema
				sm.logger.Log("debug", "Included resource: %s", resourceName)
			}
		}

		// Only add the provider schema if it has any resource schemas.
		if len(filteredProviderSchema.ResourceSchemas) > 0 {
			filteredProviderSchemas.Schemas[providerKey] = filteredProviderSchema
		}
	}

	sm.logger.Log("info", "Completed filtering provider schemas.")
	return filteredProviderSchemas
}

// RemoveComputedAttributes removes attributes that are computed and not optional or required.
func (sm *SchemaManager) RemoveComputedAttributes(providerSchemas *tfjson.ProviderSchemas) *tfjson.ProviderSchemas {
	for _, providerSchema := range providerSchemas.Schemas {
		for _, resourceSchema := range providerSchema.ResourceSchemas {
			block := resourceSchema.Block
			if block == nil {
				continue
			}

			// Remove computed-only attributes from top-level attributes.
			for attrName, attrSchema := range block.Attributes {
				if attrSchema.Computed && !attrSchema.Optional && !attrSchema.Required {
					delete(block.Attributes, attrName)
					sm.logger.Log("debug", "Removed computed-only attribute: %s", attrName)
				}
			}

			// Recursively remove computed-only attributes from nested blocks.
			for _, nestedBlock := range block.NestedBlocks {
				sm.RemoveComputedAttributesFromBlock(nestedBlock.Block)
			}
		}
	}
	return providerSchemas
}

// RemoveComputedAttributesFromBlock removes computed-only attributes from nested blocks recursively.
func (sm *SchemaManager) RemoveComputedAttributesFromBlock(block *tfjson.SchemaBlock) {
	if block == nil {
		return
	}

	// Remove computed-only attributes from this block.
	for attrName, attrSchema := range block.Attributes {
		if attrSchema.Computed && !attrSchema.Optional && !attrSchema.Required {
			delete(block.Attributes, attrName)
			sm.logger.Log("debug", "Removed computed-only attribute: %s", attrName)
		}
	}

	// Recursively process nested blocks.
	for _, nestedBlock := range block.NestedBlocks {
		sm.RemoveComputedAttributesFromBlock(nestedBlock.Block)
	}
}

// RemoveInvalidAttributesFromSchema removes invalid attributes from the schema based on validation errors.
func (sm *SchemaManager) RemoveInvalidAttributesFromSchema(cleanedSchema map[string]*tfjson.ProviderSchema, validationErrors map[string][]string) *tfjson.ProviderSchemas {
	sm.logger.Log("info", "Starting to remove invalid attributes from the schema...")

	for providerKey, providerSchema := range cleanedSchema {
		sm.logger.Log("debug", "Processing provider: %s", providerKey)

		for resourceKey, resourceSchema := range providerSchema.ResourceSchemas {
			sm.logger.Log("debug", "Processing resource: %s", resourceKey)

			// Extract the invalid attributes for this resource from validationErrors.
			var invalidAttributes []string
			for validationKey, attrs := range validationErrors {
				if strings.HasPrefix(validationKey, resourceKey) {
					invalidAttributes = attrs
					break
				}
			}

			if len(invalidAttributes) == 0 {
				sm.logger.Log("debug", "No invalid attributes found for resource: %s", resourceKey)
				continue
			}

			sm.logger.Log("debug", "Invalid attributes for resource %s: %v", resourceKey, invalidAttributes)

			// Remove the invalid attributes from the resource schema.
			for _, attrName := range invalidAttributes {
				sm.logger.Log("debug", "Attempting to remove attribute: %s from resource: %s", attrName, resourceKey)

				if _, exists := resourceSchema.Block.Attributes[attrName]; exists {
					delete(resourceSchema.Block.Attributes, attrName)
					sm.logger.Log("debug", "Removed attribute: %s from resource: %s", attrName, resourceKey)
				} else {
					sm.logger.Log("warn", "Attribute %s not found in resource %s, cannot remove", attrName, resourceKey)
				}
			}
		}
	}

	sm.logger.Log("info", "Completed removing invalid attributes from the schema.")
	return &tfjson.ProviderSchemas{
		Schemas: cleanedSchema,
	}
}
