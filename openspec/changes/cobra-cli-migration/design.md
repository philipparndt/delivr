## Context

The CLI currently dispatches subcommands via manual `os.Args` checks in `main()` and uses Go's `flag` package for each subcommand. Help text is embedded as raw string literals in Go code. The gokcat project (same author) already follows the target pattern: Cobra commands + embedded markdown help + Glamour rendering.

Current command structure:
- `delivr` (default: generate screenshots)
- `delivr rotato` (batch Rotato processing)
- `delivr rotato-frame` (pre-render Rotato frame)
- `delivr deliver` (App Store Connect upload)
- `delivr deliver list-display-types` (list ASC display types)
- `delivr version`

## Goals / Non-Goals

**Goals:**
- Replace all manual argument parsing with Cobra
- Move help text to embedded markdown files rendered with Glamour
- Follow the same patterns as gokcat (`cmd/help.go`, `cmd/styles.go`, `cmd/help/*.md`)
- Preserve all existing CLI functionality
- Add shell completion via Cobra built-in support
- Keep `generate` as the default action when no subcommand is given

**Non-Goals:**
- Changing the business logic in render, rotato, asc, or config packages
- Adding new commands or features beyond the migration
- Changing the config file format

## Decisions

### 1. File structure: one file per command

Restructure `cmd/delivr/` into:
- `main.go` — only calls `cmd.Execute()`
- `cmd/root.go` — root command, defaults to generate behavior
- `cmd/generate.go` — explicit generate subcommand
- `cmd/rotato.go` — rotato batch subcommand
- `cmd/rotato_frame.go` — rotato-frame subcommand
- `cmd/deliver.go` — deliver subcommand with `list-display-types` as a nested sub-subcommand
- `cmd/version.go` — version subcommand
- `cmd/completion.go` — shell completion subcommand
- `cmd/help.go` — `GetHelp()` function with embedded markdown
- `cmd/styles.go` — `renderMarkdown()` function using Glamour
- `cmd/help/*.md` — markdown help files

The business logic functions (`runGenerate`, `runRotatoBatch`, `runRotatoFrame`, etc.) stay in the respective command files. Alternative considered: keeping everything in two files like today — rejected because it doesn't scale and mixes concerns.

### 2. Root command doubles as generate

When the user runs `delivr --config config.yaml`, the root command acts as `generate`. This preserves backward compatibility. The explicit `delivr generate --config ...` also works. Alternative: require explicit `generate` subcommand — rejected to maintain ergonomics.

### 3. Glamour rendering with terminal detection

Same pattern as gokcat: `renderMarkdown()` checks `isTerminal()` and falls back to raw markdown when piped. This ensures help text is still readable when redirected.

### 4. Rotato subcommand with `frame` as a nested subcommand

Instead of `rotato-frame` as a top-level command, use `rotato frame` as a Cobra subcommand of `rotato`. This is more idiomatic for Cobra. The `rotato` command itself handles batch processing.

## Risks / Trade-offs

- **Breaking flag syntax** (`-config` → `--config`) → Acceptable since the tool has no external users yet. Document in README.
- **`rotato-frame` → `rotato frame`** → Slightly different invocation. Makefile needs updating.
- **Added dependencies** (cobra, glamour) → Increases binary size slightly. Acceptable tradeoff for better UX and maintainability.
