// Package terraform provides utilities to generate, manipulate, and validate
// Terraform HCL files and configurations.
package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"tmcg/internal/tmcg/logging"
	tmcgParsing "tmcg/internal/tmcg/parsing"

	"github.com/gertd/go-pluralize"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

// Tf encapsulates tf logic with logging
type Tf struct {
	logger logging.Logger
}

// NewParser creates a new Tf instance
func NewTf(logger logging.Logger) *Tf {
	return &Tf{logger: logger}
}

// ValidateTerraformBinary ensures the Terraform binary is available
var lookPath = exec.LookPath

func (t *Tf) ValidateTerraformBinary(binary string) (string, error) {
	path, err := lookPath(binary)
	if err != nil {
		return "", fmt.Errorf("terraform binary not found in PATH or specified path: %s", binary)
	}
	return path, nil
}

// CreateVersionsTF generates a versions.tf file with the required provider definitions
func (t *Tf) CreateVersionsTF(workingDir string, providers map[string]tmcgParsing.Provider) error {
	t.logger.Log("info", "Creating versions.tf...")

	// Collect keys for sorting
	keys := make([]string, 0, len(providers))
	for key := range providers {
		keys = append(keys, key)
	}
	sort.Strings(keys) // Sort keys alphabetically

	// Generate the Terraform required_providers block
	var builder strings.Builder
	builder.WriteString("terraform {\n  required_providers {\n")
	for _, key := range keys {
		provider := providers[key]
		builder.WriteString(fmt.Sprintf("    %s = {\n", provider.NameLower))
		builder.WriteString(fmt.Sprintf("      source  = \"%s/%s\"\n", provider.NamespaceLower, provider.NameLower))
		builder.WriteString(fmt.Sprintf("      version = \"%s\"\n", provider.Version))
		builder.WriteString("    }\n")
	}
	builder.WriteString("  }\n}\n")

	// Write to file
	filePath := filepath.Join(workingDir, "versions.tf")
	return os.WriteFile(filePath, []byte(builder.String()), 0644)
}

var writeFile = os.WriteFile

// CreateMainTF generates the main.tf file with resource and dynamic blocks
func (t *Tf) CreateMainTF(dir string, cleanedSchema map[string]*tfjson.ProviderSchema, resources []tmcgParsing.Resource) error {
	t.logger.Log("info", "Starting to generate main.tf in directory: %s", dir)

	// Validate inputs
	if len(resources) == 0 {
		t.logger.Log("warn", "No resources specified. Skipping main.tf generation.")
		return nil
	}

	// Create a new HCL file
	file := hclwrite.NewEmptyFile()

	// Iterate over each resource
	for _, resource := range resources {
		t.logger.Log("debug", "Processing resource: %s with provider: %s/%s", resource.Name, resource.Provider.Namespace, resource.Provider.Name)

		// Construct the provider key to access the schema
		providerKey := fmt.Sprintf("registry.terraform.io/%s/%s", resource.Provider.NamespaceLower, resource.Provider.NameLower)
		providerSchema, exists := cleanedSchema[providerKey]
		if !exists {
			t.logger.Log("warn", "No schema found for provider: %s", providerKey)
			continue
		}

		// Get the resource schema
		resourceSchema, exists := providerSchema.ResourceSchemas[resource.Name]
		if !exists {
			t.logger.Log("warn", "No schema found for resource: %s with provider: %s/%s", resource.Name, resource.Provider.Namespace, resource.Provider.Name)
			continue
		}

		// Derive the variable name
		variableName := t.deriveVariableName(resource.Name)
		t.logger.Log("debug", "Derived variable name for resource: %s", variableName)

		// Create the resource block
		resourceBlock := file.Body().AppendNewBlock("resource", []string{resource.Name, "this"})
		resourceAttrs := resourceBlock.Body()

		// Handle resource mode (single/multiple)
		if resource.Mode == "multiple" {
			// Add the `for_each` block using the derived variable name
			forEachExpression := fmt.Sprintf("{ for i in coalesce(var.%s, []) : i.name => i }", variableName)
			resourceAttrs.SetAttributeRaw("for_each", hclwrite.TokensForIdentifier(forEachExpression))
			t.logger.Log("debug", "Added for_each expression: %s", forEachExpression)
		}

		// Collect attributes and nested blocks together
		totalItems := make([]string, 0, len(resourceSchema.Block.Attributes)+len(resourceSchema.Block.NestedBlocks))
		for name := range resourceSchema.Block.Attributes {
			totalItems = append(totalItems, name)
		}
		for name := range resourceSchema.Block.NestedBlocks {
			totalItems = append(totalItems, name)
		}
		sort.Strings(totalItems)

		// Process sorted attributes and nested blocks
		for _, itemName := range totalItems {
			// Check if the item is an attribute
			if attrSchema, ok := resourceSchema.Block.Attributes[itemName]; ok {
				if resource.Mode == "single" {
					resourceAttrs.SetAttributeRaw(itemName, hclwrite.TokensForIdentifier(fmt.Sprintf("var.%s", itemName)))
					t.logger.Log("debug", "Added attribute: %s = var.%s", itemName, itemName)
				} else {
					t.handleAttributesAndNestedBlocks(resourceAttrs, map[string]*tfjson.SchemaAttribute{itemName: attrSchema}, nil, "each.value")
				}
				continue
			}

			// Otherwise, it must be a nested block
			blockSchema := resourceSchema.Block.NestedBlocks[itemName]
			if blockSchema == nil || blockSchema.Block == nil {
				t.logger.Log("warn", "Skipping invalid nested block: %s in resource: %s", itemName, resource.Name)
				continue
			}

			resourceBlock.Body().AppendNewline()
			dynamicBlock := hclwrite.NewBlock("dynamic", []string{itemName})
			dynamicBody := dynamicBlock.Body()

			// Determine the prefix based on the resource mode
			prefix := "var."
			if resource.Mode == "multiple" {
				prefix = "each.value."
			}

			dynamicBody.SetAttributeRaw("for_each", hclwrite.TokensForIdentifier(fmt.Sprintf("can(coalesce(%s%s)) ? flatten([%s%s]) : []", prefix, itemName, prefix, itemName)))

			contentBlock := hclwrite.NewBlock("content", nil)
			contentBody := contentBlock.Body()
			t.handleAttributesAndNestedBlocks(contentBody, blockSchema.Block.Attributes, blockSchema.Block.NestedBlocks, fmt.Sprintf("%s.value", itemName))

			dynamicBody.AppendBlock(contentBlock)
			resourceAttrs.AppendBlock(dynamicBlock)
			resourceBlock.Body().AppendNewline()

			t.logger.Log("debug", "Added dynamic block for nested block: %s", itemName)
		}

		// Add a newline after each resource block
		file.Body().AppendNewline()
	}

	// Write the generated file to disk
	filePath := filepath.Join(dir, "main.tf")
	t.cleanupHCLFile(file)
	t.logger.Log("info", "Writing main.tf to: %s", filePath)
	err := writeFile(filePath, file.Bytes(), 0644)
	if err != nil {
		t.logger.Log("error", "Failed to write main.tf: %v", err)
		return fmt.Errorf("failed to write main.tf to %s: %w", filePath, err)
	}

	t.logger.Log("info", "Successfully generated main.tf in directory: %s", dir)
	return nil
}

// handleAttributesAndNestedBlocks is a recursive function to handle attributes and nested blocks
func (t *Tf) handleAttributesAndNestedBlocks(resourceAttrs *hclwrite.Body, attributes map[string]*tfjson.SchemaAttribute, nestedBlocks map[string]*tfjson.SchemaBlockType, prefix string) {
	// Collect attributes and nested blocks into a combined map
	items := make(map[string]interface{}, len(attributes)+len(nestedBlocks))
	for name, attrSchema := range attributes {
		items[name] = attrSchema
	}
	for name, blockSchema := range nestedBlocks {
		items[name] = blockSchema
	}

	// Get a sorted list of item names
	itemNames := make([]string, 0, len(items))
	for name := range items {
		itemNames = append(itemNames, name)
	}
	sort.Strings(itemNames)

	// Process each item, maintaining the sorted order of attributes and nested blocks
	for _, itemName := range itemNames {
		if _, ok := items[itemName].(*tfjson.SchemaAttribute); ok {
			// Handle attribute
			resourceAttrs.SetAttributeRaw(itemName, hclwrite.TokensForIdentifier(fmt.Sprintf("%s.%s", prefix, itemName)))
			t.logger.Log("debug", "Added attribute: %s.%s", prefix, itemName)
		} else if blockSchema, ok := items[itemName].(*tfjson.SchemaBlockType); ok && blockSchema != nil {
			// Handle nested block
			t.logger.Log("debug", "Processing nested block: %s", itemName)
			resourceAttrs.AppendNewline()
			dynamicBlock := hclwrite.NewBlock("dynamic", []string{itemName})
			dynamicBody := dynamicBlock.Body()
			dynamicBody.SetAttributeRaw("for_each", hclwrite.TokensForIdentifier(fmt.Sprintf("can(coalesce(%s.%s)) ? flatten([%s.%s]) : []", prefix, itemName, prefix, itemName)))

			contentBlock := hclwrite.NewBlock("content", nil)
			contentBody := contentBlock.Body()
			t.handleAttributesAndNestedBlocks(contentBody, blockSchema.Block.Attributes, blockSchema.Block.NestedBlocks, fmt.Sprintf("%s.value", itemName))

			dynamicBody.AppendBlock(contentBlock)
			resourceAttrs.AppendBlock(dynamicBlock)
			resourceAttrs.AppendNewline()
		}
	}
}

// deriveVariableName removes the provider prefix and pluralizes the resource name
func (t *Tf) deriveVariableName(resource string) string {
	parts := strings.SplitN(resource, "_", 2)
	if len(parts) > 1 {
		resourceName := parts[1] // Get the part after provider name
		pluralizer := pluralize.NewClient()
		return pluralizer.Plural(resourceName)
	}
	return resource
}

// CreateVariablesTF generates the variables.tf file based on resource schemas
func (t *Tf) CreateVariablesTF(dir string, cleanedSchema map[string]*tfjson.ProviderSchema, resources []tmcgParsing.Resource, descAsCommentsFlag bool) error {
	t.logger.Log("info", "Starting to generate variables.tf in directory: %s", dir)

	// Validate inputs
	if len(resources) == 0 {
		t.logger.Log("warn", "No resources specified. Skipping variables.tf generation.")
		return nil
	}

	// Create a new HCL file
	file := hclwrite.NewEmptyFile()
	rootBody := file.Body()

	for _, resource := range resources {
		// Retrieve the schema for the resource
		providerKey := fmt.Sprintf("registry.terraform.io/%s/%s", resource.Provider.NamespaceLower, resource.Provider.NameLower)
		providerSchema, exists := cleanedSchema[providerKey]
		if !exists {
			t.logger.Log("warn", "No schema found for provider: %s", providerKey)
			continue
		}

		resourceSchema, exists := providerSchema.ResourceSchemas[resource.Name]
		if !exists {
			t.logger.Log("warn", "No schema found for resource: %s with provider: %s/%s", resource.Name, resource.Provider.Namespace, resource.Provider.Name)
			continue
		}

		// Derive the variable name
		variableName := t.deriveVariableName(resource.Name)

		if resource.Mode == "multiple" {
			// Handle multiple mode
			variableBlock := rootBody.AppendNewBlock("variable", []string{variableName})
			variableBody := variableBlock.Body()
			variableBody.SetAttributeRaw("type", hclwrite.TokensForIdentifier("list(object({"))

			// Process attributes and nested blocks
			t.handleAttributesAndNestedBlocksForVariable(variableBody, resourceSchema.Block.Attributes, resourceSchema.Block.NestedBlocks, 1, true, descAsCommentsFlag)

			// Close the variable type definition
			variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenIdent, Bytes: []byte("}))")},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
			})

			variableBody.SetAttributeRaw("default", hclwrite.TokensForIdentifier("null"))
			rootBody.AppendNewline()
		} else {
			// Handle single mode
			totalItems := make([]string, 0, len(resourceSchema.Block.Attributes)+len(resourceSchema.Block.NestedBlocks))
			for name := range resourceSchema.Block.Attributes {
				totalItems = append(totalItems, name)
			}
			for name := range resourceSchema.Block.NestedBlocks {
				totalItems = append(totalItems, name)
			}
			sort.Strings(totalItems)

			for _, itemName := range totalItems {
				// Check if it's an attribute
				if attrSchema, ok := resourceSchema.Block.Attributes[itemName]; ok {
					if attrSchema == nil {
						t.logger.Log("debug", "Skipping attribute: %s", itemName)
						continue
					}

					variableBlock := rootBody.AppendNewBlock("variable", []string{itemName})
					variableBody := variableBlock.Body()

					// Set description

					if description := strings.ReplaceAll(attrSchema.Description, "\n", " "); description != "" {
						variableBody.SetAttributeValue("description", cty.StringVal(description))
					}

					// Set type and default
					attrTypeStr := t.getAttributeType(attrSchema.AttributeType)
					variableBody.SetAttributeRaw("type", hclwrite.TokensForIdentifier(attrTypeStr))
					if attrSchema.Optional {
						variableBody.SetAttributeRaw("default", hclwrite.TokensForIdentifier("null"))
					}
					rootBody.AppendNewline()
					continue
				}

				// Handle nested blocks
				block := resourceSchema.Block.NestedBlocks[itemName]
				if block == nil || block.Block == nil {
					t.logger.Log("warn", "Skipping invalid nested block: %s", itemName)
					continue
				}

				variableBlock := rootBody.AppendNewBlock("variable", []string{itemName})
				variableBody := variableBlock.Body()

				// Determine block type
				typeStr := "object({"
				if block.MaxItems != 1 {
					typeStr = "list(object({"
				}
				variableBody.SetAttributeRaw("type", hclwrite.TokensForIdentifier(typeStr))

				// Process nested attributes and blocks
				t.handleAttributesAndNestedBlocksForVariable(variableBody, block.Block.Attributes, block.Block.NestedBlocks, 1, true, descAsCommentsFlag)

				// Close block
				closingString := "})"
				if block.MaxItems != 1 {
					closingString = "}))"
				}
				variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
					{Type: hclsyntax.TokenIdent, Bytes: []byte(closingString)},
					{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				})
				rootBody.AppendNewline()

				// Set default for optional blocks
				if block.MinItems == 0 {
					variableBody.SetAttributeRaw("default", hclwrite.TokensForIdentifier("null"))
				}
			}
		}
	}

	// Write to disk
	filePath := filepath.Join(dir, "variables.tf")
	t.cleanupHCLFile(file)
	t.logger.Log("info", "Writing variables.tf to: %s", filePath)
	err := writeFile(filePath, file.Bytes(), 0644)

	if err != nil {
		t.logger.Log("error", "Failed to write variables.tf: %v", err)
		return fmt.Errorf("failed to write variables.tf to %s: %w", filePath, err)
	}

	t.logger.Log("info", "Successfully generated variables.tf in directory: %s", dir)
	return nil
}

// handleAttributesAndNestedBlocksForVariable is a recursive function to handle attributes and nested blocks for variable definitions
func (t *Tf) handleAttributesAndNestedBlocksForVariable(variableBody *hclwrite.Body, attributes map[string]*tfjson.SchemaAttribute, nestedBlocks map[string]*tfjson.SchemaBlockType, indentLevel int, isNested bool, descAsCommentsFlag bool) {
	indent := strings.Repeat("  ", indentLevel)

	type schemaItem struct {
		Name   string
		IsAttr bool
	}
	items := make([]schemaItem, 0, len(attributes)+len(nestedBlocks))

	for name := range attributes {
		items = append(items, schemaItem{Name: name, IsAttr: true})
	}

	for name := range nestedBlocks {
		items = append(items, schemaItem{Name: name, IsAttr: false})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	for _, item := range items {
		if item.IsAttr {
			// Handle attributes
			attrName := item.Name
			attrSchema := attributes[attrName]

			// Resolve attribute type
			attrTypeStr := t.getAttributeType(attrSchema.AttributeType)

			// Add description comment if available
			if attrSchema.Description != "" && descAsCommentsFlag {
				escapedDescription := strings.ReplaceAll(attrSchema.Description, `"`, `\"`)
				singleLineDescription := strings.ReplaceAll(escapedDescription, "\n", " ")
				variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
					{Type: hclsyntax.TokenComment, Bytes: []byte(fmt.Sprintf("%s// %s", indent, singleLineDescription))},
					{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				})
			}

			optionalPrefix := ""
			optionalSuffix := ""

			// Wrap with optional() if attribute is optional and nested
			if !attrSchema.Required && isNested {
				optionalPrefix = "optional("
				optionalSuffix = ")"
			}

			// Format the attribute
			formattedAttribute := fmt.Sprintf("%s%s = %s%s%s", indent, attrName, optionalPrefix, attrTypeStr, optionalSuffix)
			t.logger.Log("debug", "Adding attribute: %s = %s%s%s", attrName, optionalPrefix, attrTypeStr, optionalSuffix)

			// Add the attribute line
			variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenIdent, Bytes: []byte(formattedAttribute)},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
			})
		} else {
			// Handle nested blocks
			variableBody.AppendNewline()
			blockName := item.Name
			blockSchema := nestedBlocks[blockName]

			// Determine block type
			var blockTypeStr string
			switch blockSchema.NestingMode {
			case "single":
				blockTypeStr = "object({"
			case "list":
				blockTypeStr = "list(object({"
			case "set":
				blockTypeStr = "set(object({"
			default:
				t.logger.Log("warn", "Unknown nesting_mode for block %s. Skipping.", blockName)
				continue
			}

			t.logger.Log("debug", "Processing nested block: %s", blockName)

			// Add block type and optionality
			isOptional := blockSchema.MinItems == 0
			if isOptional {
				variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
					{Type: hclsyntax.TokenIdent, Bytes: []byte(fmt.Sprintf("%s%s = optional(%s", indent, blockName, blockTypeStr))},
					{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				})
			} else {
				variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
					{Type: hclsyntax.TokenIdent, Bytes: []byte(fmt.Sprintf("%s%s = %s", indent, blockName, blockTypeStr))},
					{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				})
			}

			// Add description comment if available
			if blockSchema.Block.Description != "" && descAsCommentsFlag {
				escapedDescription := strings.ReplaceAll(blockSchema.Block.Description, `"`, `\"`)
				singleLineDescription := strings.ReplaceAll(escapedDescription, "\n", " ")
				variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
					{Type: hclsyntax.TokenComment, Bytes: []byte(fmt.Sprintf("%s  // %s", indent, singleLineDescription))},
					{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				})
			}

			// Recursively process nested attributes and blocks
			t.handleAttributesAndNestedBlocksForVariable(
				variableBody,
				blockSchema.Block.Attributes,
				blockSchema.Block.NestedBlocks,
				indentLevel+1,
				true,
				descAsCommentsFlag,
			)

			// Close block type
			closeStr := fmt.Sprintf("%s}))", indent)
			if blockSchema.NestingMode == "single" {
				closeStr = fmt.Sprintf("%s})", indent)
			}

			// Add closing parentheses for optional blocks
			if isOptional {
				closeStr += ")"
			}

			variableBody.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenIdent, Bytes: []byte(closeStr)},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
			})
			variableBody.AppendNewline()
		}
	}
}

// getAttributeType returns the Terraform type string representation for a given cty.Type
func (t *Tf) getAttributeType(attrType cty.Type) string {
	switch {
	case attrType.IsPrimitiveType():
		return attrType.FriendlyName()
	case attrType.IsListType():
		elementType := t.getAttributeType(attrType.ElementType())
		return fmt.Sprintf("list(%s)", elementType)
	case attrType.IsSetType():
		elementType := t.getAttributeType(attrType.ElementType())
		return fmt.Sprintf("set(%s)", elementType)
	case attrType.IsMapType():
		mapElementType := t.getAttributeType(*attrType.MapElementType())
		return fmt.Sprintf("map(%s)", mapElementType)
	case attrType.IsObjectType():
		var builder strings.Builder
		builder.WriteString("object({\n")
		for key, val := range attrType.AttributeTypes() {
			attributeType := t.getAttributeType(val)
			builder.WriteString(fmt.Sprintf("  %s = %s\n", key, attributeType))
		}
		builder.WriteString("})")
		return builder.String()
	default:
		return "any"
	}
}

type TerraformExecutor interface {
	Validate(ctx context.Context) (*tfjson.ValidateOutput, error)
	WorkingDir() string
}

// RunTerraformValidate executes terraform validate and capture any errors in JSON format
func (t *Tf) RunTerraformValidate(tf TerraformExecutor) (map[string][]string, error) {
	// Log the directory where validation is supposed to run
	t.logger.Log("info", "Running terraform validate in directory: %s", tf.WorkingDir())

	// Run terraform validate
	t.logger.Log("debug", "Executing terraform validate command...")
	validateOutput, err := tf.Validate(context.Background())
	if err != nil {
		t.logger.Log("warn", "Terraform validate command failed: %v", err)
	}

	// Marshal ValidateOutput to JSON
	validateJSON, err := json.Marshal(validateOutput)
	if err != nil {
		t.logger.Log("error", "Failed to marshal validate output to JSON: %v", err)
		return nil, fmt.Errorf("failed to marshal validate output to JSON: %w", err)
	}

	// Log the validation output based on log level
	switch logging.GetLogLevel() {
	case "debug":
		t.logger.Log("debug", "Validation output: %s", string(validateJSON))
	case "info":
		var validationOutput map[string]interface{}
		if err := json.Unmarshal(validateJSON, &validationOutput); err != nil {
			t.logger.Log("warn", "Failed to parse validation JSON: %v", err)
		} else if valid, exists := validationOutput["valid"]; exists {
			t.logger.Log("info", "Validation result: valid = %v", valid)
		}
	}

	// Parse validation errors from JSON output
	t.logger.Log("debug", "Parsing validation errors from terraform validate output...")
	logger := logging.GetGlobalLogger()
	parser := tmcgParsing.NewParser(logger)
	parsedErrors, parseErr := parser.ParseValidationErrorsFromJSON(string(validateJSON))
	if parseErr != nil {
		t.logger.Log("error", "Failed to parse validation errors: %v", parseErr)
		return nil, fmt.Errorf("failed to parse validation errors from terraform validate output: %w", parseErr)
	}

	// Return parsed errors and log results
	if len(parsedErrors) > 0 {
		t.logger.Log("warn", "Terraform validate completed with %d errors.", len(parsedErrors))
	} else {
		t.logger.Log("info", "Terraform validate completed successfully with no errors.")
	}

	return parsedErrors, nil
}

// RunTerraformFmt runs the `terraform fmt` command in the specified directory
type TerraformFmtFunc func(ctx context.Context, opts ...tfexec.FormatOption) error

func (t *Tf) RunTerraformFmt(tfWorkingDir string, fmtFunc TerraformFmtFunc, opts ...tfexec.FormatOption) error {
	ctx := context.Background()

	t.logger.Log("debug", "Running terraform fmt in directory: %s", tfWorkingDir)

	// Run terraform fmt using the provided function
	err := fmtFunc(ctx, opts...)
	if err != nil {
		t.logger.Log("error", "Failed to run terraform fmt: %v", err)
		return fmt.Errorf("failed to run terraform fmt: %w", err)
	}

	t.logger.Log("info", "Terraform fmt completed successfully.")
	return nil
}

// cleanupHCLFile processes the HCL content to apply cleanup rules.
func (t *Tf) cleanupHCLFile(file *hclwrite.File) {
	// Extract the raw content of the file's body
	content := string(file.Body().BuildTokens(nil).Bytes())

	// Replace 2 or more consecutive empty lines with a single empty line
	content = regexp.MustCompile(`(?m)(^\s*$\n){2,}`).ReplaceAllString(content, "\n")

	// Remove empty lines before closing braces
	content = regexp.MustCompile(`(?m)(^\s*$\n){1,}(\s*})`).ReplaceAllString(content, "$2")

	// Clear the body of the HCL file
	file.Body().Clear()

	// Write the cleaned content back line by line
	for _, line := range strings.Split(content, "\n") {
		file.Body().AppendUnstructuredTokens(hclwrite.TokensForIdentifier(line))
		file.Body().AppendNewline() // Ensure proper formatting
	}
}
