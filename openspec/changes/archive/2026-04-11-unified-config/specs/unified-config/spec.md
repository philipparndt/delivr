## ADDED Requirements

### Requirement: Root delivr.yaml config format
The tool SHALL support a root `delivr.yaml` config that contains shared `settings`, `devices`, and sections for `capture`, `generate`, and `deliver`. Each command SHALL read its relevant section from the root config.

#### Scenario: Generate reads from root config
- **WHEN** user runs `delivr generate --config delivr.yaml`
- **THEN** the command reads the `generate` section, `devices`, and `settings` from the root config

### Requirement: Auto-detect config type
The tool SHALL auto-detect whether a config file is a root config (has `capture`/`generate`/`deliver` keys) or a standalone per-command config, and handle both.

#### Scenario: Standalone config still works
- **WHEN** user runs `delivr capture --config capture.yaml` with a standalone capture config (no root-level `capture` key)
- **THEN** the command reads the config as a standalone capture config

### Requirement: Shared devices section
The `devices` section in the root config SHALL be used by all commands. Device entries SHALL include both capture-relevant fields (`platform`) and generate-relevant fields (`width`, `height`, `display_type`, `screenshot_prefix`).

#### Scenario: Devices used by capture and generate
- **WHEN** a device defines `platform: ios`, `width: 1320`, `height: 2868`
- **THEN** `capture` uses the `platform` and `name` fields, and `generate` uses `width`, `height`, and `screenshot_prefix`

### Requirement: Init command scaffolds a project
The CLI SHALL provide a `delivr init` command that interactively creates a project configuration. It SHALL prompt the user to select devices, then generate a set of example config files including `delivr.yaml`, `devices.yaml`, example screen templates, and example App Store text markdown files.

#### Scenario: Init creates project scaffold
- **WHEN** user runs `delivr init`
- **THEN** the command prompts for device selection and generates `delivr.yaml`, `devices.yaml`, `templates.yaml`, `screens/example.yaml`, `metadata/en-US/description.md`, `metadata/en-US/promotional_text.md`, `metadata/en-US/release_notes.md`, and `.claude/commands/` with AI helper commands

### Requirement: Init generates Claude Code commands
The `delivr init` command SHALL generate `.claude/commands/` files for common workflows: generating changelogs from git history, updating App Store description text, and translating App Store texts to configured languages.

#### Scenario: Claude commands created
- **WHEN** `delivr init` completes
- **THEN** `.claude/commands/changelog.md`, `.claude/commands/update-description.md`, and `.claude/commands/translate.md` are created with appropriate prompts
