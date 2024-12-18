# Terraform Module Code Generator

### Build and Test Status
[![Build Status](https://github.com/kbcz1989/tmcg/actions/workflows/release.yml/badge.svg)](https://github.com/kbcz1989/tmcg/actions/workflows/release.yml)
[![Tests](https://img.shields.io/github/actions/workflow/status/kbcz1989/tmcg/tests.yml?label=tests)](https://github.com/kbcz1989/tmcg/actions/workflows/tests.yml)


### Versioning and Downloads
![Go Version](https://img.shields.io/github/go-mod/go-version/kbcz1989/tmcg)
![Latest Release](https://img.shields.io/github/v/release/kbcz1989/tmcg)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/kbcz1989/tmcg)
![Downloads](https://img.shields.io/github/downloads/kbcz1989/tmcg/latest/total)

### Code Quality and Metrics
[![Go Report Card](https://goreportcard.com/badge/github.com/kbcz1989/tmcg)](https://goreportcard.com/report/github.com/kbcz1989/tmcg)
[![Codecov](https://codecov.io/gh/kbcz1989/tmcg/branch/main/graph/badge.svg)](https://codecov.io/gh/kbcz1989/tmcg)

![Build Size](https://img.shields.io/github/languages/code-size/kbcz1989/tmcg)
![Languages](https://img.shields.io/github/languages/top/kbcz1989/tmcg)

### Community and Contributions
![Open Issues](https://img.shields.io/github/issues/kbcz1989/tmcg)
![Contributors](https://img.shields.io/github/contributors/kbcz1989/tmcg)
![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)

### License and Miscellaneous
![License](https://img.shields.io/github/license/kbcz1989/tmcg)
![Markdown Style](https://img.shields.io/badge/markdown-friendly-yellow)
![Powered by Go](https://img.shields.io/badge/powered%20by-Go-blue?logo=go)
![Last Commit](https://img.shields.io/github/last-commit/kbcz1989/tmcg)


A Go-based tool for dynamically generating Terraform modules, including `main.tf`, `variables.tf`, and `versions.tf`, by leveraging Terraform schemas and user-defined resource configurations.

## Features

- Parses and validates Terraform providers and resources.
- Dynamically generates `main.tf`, `variables.tf`, and `versions.tf` files.
- Supports filtering Terraform provider schemas for required resources.
- Removes computed and invalid attributes from schemas.
- Ensures proper HCL formatting using `terraform fmt`.
- Logs detailed messages at various levels (info, debug, warn, error).

## Requirements

- Terraform binary installed (v1.3+)

### For installation from source

- Go 1.23+
- `go.mod` dependencies (e.g., `terraform-json`, `terraform-exec`, `zclconf/go-cty`).

## Installation

### Installation with asdf version manager
```shell
# Add the plugin
asdf plugin add tmcg https://github.com/kbcz1989/asdf-tmcg.git

# Install a version
asdf install tmcg latest

# Set it globally
asdf global tmcg latest

# Verify installation
tmcg --version
```

---

### Manual installation

### Linux:

```shell
ARCH=$(uname -m | grep -q 'aarch64' && echo 'arm64' || echo 'amd64')
sudo wget "https://github.com/kbcz1989/tmcg/releases/latest/download/tmcg-linux-$ARCH" -O /usr/local/bin/tmcg
sudo chmod +x /usr/local/bin/tmcg
```

### macOS:

```shell
ARCH=$(uname -m | grep -q 'arm64' && echo 'arm64' || echo 'amd64')
curl -L "https://github.com/kbcz1989/tmcg/releases/latest/download/tmcg-darwin-$ARCH" -o /usr/local/bin/tmcg
chmod +x /usr/local/bin/tmcg
```

### Windows:

```shell
$ARCH = if ($ENV:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
Invoke-WebRequest -Uri "https://github.com/kbcz1989/tmcg/releases/latest/download/tmcg-windows-$ARCH.exe" -OutFile "$Env:LOCALAPPDATA\tmcg.exe" -UseBasicParsing
```

## Installation from Source

Clone the repository:

```bash
git clone https://github.com/kbcz1989/tmcg.git
cd tmcg-generator
```

Build the project:

```bash
go build -o tmcg
```

Run the binary:

```bash
./tmcg --help
```

## Usage

### Command-Line Options

| Flag                | Description                                                                         | Example                       |
| ------------------- | ----------------------------------------------------------------------------------- | ----------------------------- |
| `--provider, -p`    | Specify Terraform providers (e.g., `hashicorp/aws:>=3.0`).                          | `-p hashicorp/aws:>=3.0`      |
| `--resource, -r`    | Specify resources (e.g., `aws_instance:single`, `azurerm_resource_group:multiple`). | `-r aws_instance:single`      |
| `--directory, -d`   | The working directory for Terraform files.                                          | `-d ./output`                 |
| `--binary, -b`      | The path to the Terraform binary.                                                   | `-b /usr/local/bin/terraform` |
| `--log-level, -l`   | Set the log level (e.g., `debug`, `info`, `warn`, `error`).                         | `-l debug`                    |
| `--help, -h`        | Show usage information.                                                             |                               |
| `--version, -v`     | Show app version.                                                                   |                               |
| `--desc-as-comment` | Include the description as a comment in multiple mode.                              | `--desc-as-comment=true`      |

### Example Command

```bash
./tmcg -p hashicorp/aws:>=3.0 -r aws_instance:single -d ./output -l debug
```

### Output Files

- **`main.tf`**: Contains resource definitions with dynamic blocks.
- **`variables.tf`**: Defines input variables for the resources.
- **`versions.tf`**: Specifies required providers and their versions.

## Tests

Run all tests:

```bash
make test
```

### Key Test Features

- Verifies resource and provider parsing.
- Ensures `main.tf`, `variables.tf`, and `versions.tf` generation matches expectations.
- Validates cleanup of HCL files and schema handling.

## How `tmcg` Works

1. **Parsing Provider and Resource Arguments**
   - The tool takes arguments for providers and resources passed via CLI flags.
   - Providers can be specified with optional versions (e.g., `hashicorp/aws:>=3.0`).
   - Resources are parsed with an optional mode (`single` or `multiple`), defaulting to `multiple`.

2. **Building `versions.tf`**
   - The tool constructs a `versions.tf` file based on the provided provider arguments.
   - The file specifies provider sources and version constraints.

3. **Running `terraform init`**
   - Once the `versions.tf` file is created, `tmcg` runs `terraform init` to download required providers.
   - This ensures that all dependencies are in place before schema processing.

4. **Fetching Terraform Provider Schema**
   - The tool fetches the full schema for the specified providers using `terraform providers schema`.
   - This includes details of attributes, blocks, and their metadata (e.g., required, computed, optional).

5. **Filtering the Schema for Parsed Resources**
   - From the fetched schema, only the resources specified via CLI arguments are retained.
   - This narrows down the schema to the required subset.

6. **Removing Computed-Only Attributes**
   - The filtered schema is further refined by removing attributes that are only computed and cannot be configured by users.

7. **Generating `main.tf` and `variables.tf`**
   - Based on the filtered schema, `tmcg` generates:
     - **`main.tf`**: A skeleton configuration for resources with all possible attributes.
     - **`variables.tf`**: A file specifying the input variable types for Terraform.

8. **Running `terraform validate`**
   - After generating the configuration files, `tmcg` runs `terraform validate` to check for errors or warnings.
   - This step helps identify any invalid or unsupported attributes.

9. **Removing Invalid Attributes**
   - If validation errors are detected, `tmcg` analyzes the errors and removes invalid attributes from the filtered schema.
   - The process ensures the configuration adheres to the schema and Terraform constraints.

10. **Re-running Generation**
    - With the refined schema, `tmcg` regenerates `main.tf` and `variables.tf`.
    - This ensures the configurations are correct after invalid attributes are removed.

11. **Running `terraform fmt`**
    - The tool formats the generated Terraform configuration files for better readability and consistency.

12. **Final Validation**
    - A final `terraform validate` is executed to confirm the validity of the configurations.

---

## Real-World Scenario

Imagine you are tasked with writing a Terraform module for deploying resources, such as AWS instances or Azure resources. Crafting such modules can be a daunting task because:

1. **You need to understand the resource schema in-depth or read docs line by line**: Terraform provider resources often support dozens of attributes, some required, others optional, and many computed.
2. **Ensuring flexibility**: To make a module reusable, you must expose configurable parameters for all relevant resource attributes.
3. **Keeping up with upstream changes**: Terraform providers are frequently updated, and your module must remain compatible with the latest schema.

This is where `tmcg` can be helpful.

### How `tmcg` Solves This Problem

`tmcg` generates Terraform module boilerplate code by leveraging Terraform provider schemas directly. Its approach is guided by the author's subjective best practices, emphasizing flexibility and extensibility:

- **Comprehensive attribute support**: All attributes supported by the upstream provider are included in the generated code, ensuring you don’t miss any critical options.
- **Baseline for further customization**: The generated module serves as a starting point, which you can refine and extend to meet specific requirements.
- **Automated validation and cleanup**: It automatically validates and filters out computed-only attributes that should not be user-configurable.

For example, if you want to create a module for an AWS EC2 instance, instead of manually crafting the `variables.tf` and `main.tf`, you can run `tmcg` and instantly get:

1. A `variables.tf` with all attributes defined and ready for customization.
2. A `main.tf` implementing these variables with proper references.
3. A `versions.tf` that enforces the required providers and Terraform version.

### Example Workflow

#### 1. Define Resources and Providers

Use `tmcg` to specify resources and providers for your module. For instance:

```bash
tmcg --resource aws_instance:single --provider "hashicorp/aws:>=4.0" --directory my-module
```

#### 2. Review and Expand

Navigate to the `my-module` directory, where you’ll find:

- `versions.tf`: Ensures compatibility with the required Terraform version and provider version.
- `variables.tf`: Includes all possible attributes for the `aws_instance` resource.
- `main.tf`: Implements the logic based on the variables.

From here, you can further tailor the module to fit your use case.

#### 3. Validate and Apply

Run Terraform commands to validate and test the module:

```bash
terraform init
terraform validate
```

`tmcg` ensures the code is clean and adheres to best practices, reducing manual effort and errors.

### Benefits

- **Time-saving**: Quickly generate a baseline for Terraform modules, avoiding repetitive tasks.
- **Up-to-date**: Leverage the latest upstream provider schemas to ensure compatibility with the newest features.
- **Customization-friendly**: The generated code is easy to expand and customize, making it suitable for real-world use.

### Real-World Use Case

A DevOps/TechOps engineer responsible for setting up cloud infrastructure can use `tmcg` to:

1. Generate a module for managing EC2 instances.
2. Quickly iterate over configurations for different environments (e.g., staging, production) by modifying the generated `variables.tf`.
3. Stay compatible with the latest AWS provider by regenerating the module as provider schemas evolve.

By streamlining the module creation process, `tmcg` lets you focus on high-value tasks like optimizing resource configurations and scaling infrastructure.

---

## Contributing

Contributions are welcome! Feel free to fork the repository and submit pull requests.

## Acknowledgments

Special thanks to:

- [@benjy44](https://github.com/benjy44) and [@hampusrosvall](https://github.com/hampusrosvall) for inspiring this project.
- [Terraform](https://www.terraform.io) for its amazing tooling and APIs.
- [Go](https://golang.org) community for the fantastic libraries used in this project.
- **ChatGPT** for guiding and assisting in creating this tool and documentation.

