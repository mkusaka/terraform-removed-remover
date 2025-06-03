# Examples

This directory contains example Terraform files that demonstrate the usage of terraform-removed-remover.

## Files

- `main.tf` - Basic example with resources and removed blocks
- `modules/` - Example module structure with removed blocks
- `edge_cases/` - Special cases and edge scenarios

## Usage

To test the tool with these examples:

```bash
# Dry run to see what would be changed
terraform-removed-remover -dry-run -verbose ./examples

# Actually remove the removed blocks
terraform-removed-remover -verbose ./examples

# Remove with whitespace normalization
terraform-removed-remover -normalize-whitespace ./examples
```

## Expected Behavior

The tool will:
1. Recursively scan all `.tf` files in the examples directory
2. Remove all `removed` blocks from the files
3. Apply standard Terraform formatting
4. Report statistics about the changes made

After running the tool, all `removed` blocks should be completely removed from the Terraform files while preserving all other content and proper formatting.
