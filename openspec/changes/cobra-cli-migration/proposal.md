## Why

The CLI currently uses manual `os.Args` dispatch and `flag.FlagSet` for each subcommand, resulting in fragile argument parsing and large inline help strings. Migrating to Cobra provides proper subcommand routing, flag inheritance, and shell completion. Using embedded markdown files with Glamour (as done in gokcat) gives rich, readable help output in the terminal while keeping help content maintainable.

## What Changes

- Replace manual `os.Args` dispatch in `main()` with Cobra root command and subcommands
- Replace all `flag`/`flag.FlagSet` usage with Cobra persistent and local flags
- Move all inline help text from Go string literals to embedded markdown files (`cmd/help/*.md`)
- Add Glamour-based markdown rendering for help output (with terminal detection fallback)
- Add `generate`, `rotato`, `rotato-frame`, `deliver`, `deliver list-display-types`, and `version` as proper Cobra subcommands
- Make `generate` the default command (root command behavior when config is provided)
- Add shell completion support via Cobra's built-in `completion` subcommand
- **BREAKING**: Flag syntax changes from `-flag` to `--flag` (Cobra uses POSIX-style flags)

## Capabilities

### New Capabilities
- `cobra-commands`: Cobra command structure with root, generate, rotato, rotato-frame, deliver, and version subcommands
- `markdown-help`: Embedded markdown help files rendered with Glamour for terminal output

### Modified Capabilities

## Impact

- **Code**: `cmd/delivr/main.go` and `cmd/delivr/deliver.go` are restructured into separate command files
- **Dependencies**: Adds `github.com/spf13/cobra`, `github.com/charmbracelet/glamour`
- **CLI interface**: Flag style changes from `-flag` to `--flag`; subcommand routing becomes explicit
- **Users**: `delivr generate --config ...` replaces `delivr -config ...`; `delivr --help` shows rendered markdown
