## ADDED Requirements

### Requirement: Help text stored as embedded markdown files
Each command's long help text SHALL be stored in a separate markdown file under `cmd/help/` and embedded at compile time using Go's `//go:embed` directive.

#### Scenario: Help files are embedded
- **WHEN** the binary is built
- **THEN** all markdown help files from `cmd/help/` are compiled into the binary

### Requirement: Markdown rendered in terminal with Glamour
When the output is a terminal, help text SHALL be rendered using Glamour with auto-style detection and word wrap at 100 characters. When output is piped (not a terminal), raw markdown SHALL be returned.

#### Scenario: Terminal help output is styled
- **WHEN** user runs `delivr --help` in a terminal
- **THEN** help text is rendered with Glamour styling (colors, formatting)

#### Scenario: Piped help output is raw markdown
- **WHEN** user runs `delivr --help | cat`
- **THEN** help text is output as plain markdown without ANSI escape codes

### Requirement: Each command has a markdown help file
The following markdown help files SHALL exist: `root.md`, `generate.md`, `rotato.md`, `rotato-frame.md`, `deliver.md`, `version.md`, `completion.md`.

#### Scenario: Help file coverage
- **WHEN** any command's `--help` flag is used
- **THEN** the corresponding markdown file's content is rendered as the long description
