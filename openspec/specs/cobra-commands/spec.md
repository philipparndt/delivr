## ADDED Requirements

### Requirement: Root command with generate as default
The root command SHALL accept `--config`, `--output`, and `--verbose` flags. When invoked with `--config`, it SHALL execute screenshot generation (same as the `generate` subcommand).

#### Scenario: Root command generates screenshots
- **WHEN** user runs `delivr --config config.yaml --output ./out`
- **THEN** screenshots are generated using the provided config, identical to `delivr generate`

### Requirement: Generate subcommand
The CLI SHALL provide a `generate` subcommand that accepts `--config` (required), `--output` (default `./output`), and `--verbose` flags and generates App Store screenshots.

#### Scenario: Explicit generate subcommand
- **WHEN** user runs `delivr generate --config config.yaml`
- **THEN** screenshots are generated to the default output directory

### Requirement: Rotato subcommand for batch processing
The CLI SHALL provide a `rotato` subcommand that accepts `--template` (required), `--images` (required), `--output` (required), `--frames` (optional), and `--verbose` flags.

#### Scenario: Batch process through Rotato
- **WHEN** user runs `delivr rotato --template scene.rotato --images ./screenshots --output ./3d`
- **THEN** all images are processed through the Rotato template

### Requirement: Rotato frame as nested subcommand
The CLI SHALL provide `rotato frame` as a subcommand of `rotato` that accepts `--template` (required), `--output` (required), `--dim` (default `1320x2868`), and `--verbose` flags.

#### Scenario: Pre-render Rotato frame
- **WHEN** user runs `delivr rotato frame --template iphone_front.rotato --output frames`
- **THEN** a reusable frame PNG and JSON metadata are generated

### Requirement: Deliver subcommand for App Store Connect
The CLI SHALL provide a `deliver` subcommand that accepts `--config` (required), `--screenshots`, `--metadata`, `--key-id`, `--issuer-id`, `--key-file`, `--key-pem`, `--skip-metadata`, `--skip-screenshots`, and `--verbose` flags. Credentials SHALL fall back to environment variables (`ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY_FILE`, `ASC_KEY_PEM`).

#### Scenario: Upload to App Store Connect
- **WHEN** user runs `delivr deliver --config appstore.yaml`
- **THEN** metadata and screenshots are uploaded to App Store Connect

### Requirement: Deliver list-display-types as nested subcommand
The CLI SHALL provide `deliver list-display-types` as a subcommand of `deliver`.

#### Scenario: List display types
- **WHEN** user runs `delivr deliver list-display-types --config appstore.yaml`
- **THEN** available display types for the bundle ID are listed

### Requirement: Version subcommand
The CLI SHALL provide a `version` subcommand that prints the version, git commit, and build date.

#### Scenario: Show version info
- **WHEN** user runs `delivr version`
- **THEN** output shows `delivr <version> (commit: <hash>, built: <date>)`

### Requirement: Shell completion subcommand
The CLI SHALL provide a `completion` subcommand that generates shell completion scripts for bash, zsh, fish, and powershell.

#### Scenario: Generate bash completion
- **WHEN** user runs `delivr completion bash`
- **THEN** a valid bash completion script is printed to stdout
