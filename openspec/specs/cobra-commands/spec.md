## MODIFIED Requirements

### Requirement: Rotato subcommand for batch processing
The `delivr rotato` command SHALL accept `--input` (required, directory or list of `.rotato` files), `--output` (required, template output directory), `--dim` (optional, default `1320x2868`), and `--verbose` flags. It generates device template sets from `.rotato` files.

#### Scenario: Generate templates from .rotato files
- **WHEN** user runs `delivr rotato --input ./rotato-files --output ./frames`
- **THEN** device template sets are generated for each `.rotato` file in the input directory

## REMOVED Requirements

### Requirement: Rotato frame as nested subcommand
**Reason**: The `rotato frame` subcommand's functionality is absorbed into the reworked `rotato` command, which now generates templates for multiple .rotato files at once.
**Migration**: Use `delivr rotato --input <dir> --output <dir>` instead of `delivr rotato frame --template <file> --output <dir>`.
