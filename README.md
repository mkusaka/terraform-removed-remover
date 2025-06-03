# Terraform Removed Block Remover

A Go tool that recursively scans Terraform files and removes all `removed` blocks.

## Overview

This tool helps clean up Terraform configurations by removing all `removed` directives, which are typically used during resource lifecycle management but may not be needed after changes are applied.

## Features

- Recursively scans directories for `.tf` files
- Identifies and removes all `removed` blocks
- Applies standard Terraform formatting to files
- Modifies files in-place
- Reports detailed statistics about the changes made
- Uses Terraform's HCL parser for accurate syntax handling

## Requirements

- Go 1.24 or later

## Installation

### From Source

```bash
git clone https://github.com/mkusaka/terraform-removed-remover.git
cd terraform-removed-remover
go build -o terraform-removed-remover cmd/terraform-removed-remover/main.go
```

### Using Go Install

```bash
go install github.com/mkusaka/terraform-removed-remover/cmd/terraform-removed-remover@latest
```

This will install the binary in your `$GOPATH/bin` directory.

## Usage

```bash
./terraform-removed-remover [options] [directory]
```

If directory is not specified, the current directory will be used.

### Options

- `-help`: Display help information
- `-version`: Display version information
- `-dry-run`: Run without modifying files
- `-verbose`: Enable verbose output
- `-normalize-whitespace`: Control whitespace normalization after removing removed blocks (default: false)

### Example

```bash
./terraform-removed-remover -verbose ./terraform
```

This will:
1. Scan all `.tf` files in the `./terraform` directory and its subdirectories
2. Remove all `removed` blocks from these files
3. Display statistics about the changes made

## Example Output

```
Scanning directory: ./terraform
Found 15 Terraform files

Statistics:
Files processed: 15
Files modified: 7
Removed blocks removed: 12
Processing time: 235.412ms
```

## How It Works

The tool uses HashiCorp's HCL library to parse Terraform files and manipulate the Abstract Syntax Tree (AST). This ensures proper handling of Terraform's syntax and maintains formatting of the files.

## License

MIT
