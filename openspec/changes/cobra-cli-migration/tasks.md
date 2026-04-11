## 1. Dependencies and Scaffolding

- [x] 1.1 Add `github.com/spf13/cobra` and `github.com/charmbracelet/glamour` dependencies
- [x] 1.2 Create `cmd/` package directory structure (`cmd/` alongside `cmd/delivr/`)
- [x] 1.3 Create `cmd/help.go` with `//go:embed` and `GetHelp()` function
- [x] 1.4 Create `cmd/styles.go` with `renderMarkdown()` and `isTerminal()` functions

## 2. Markdown Help Files

- [x] 2.1 Create `cmd/help/root.md` with root command help text
- [x] 2.2 Create `cmd/help/generate.md` with generate command help text
- [x] 2.3 Create `cmd/help/rotato.md` with rotato batch processing help text
- [x] 2.4 Create `cmd/help/rotato-frame.md` with rotato frame pre-rendering help text
- [x] 2.5 Create `cmd/help/deliver.md` with deliver command help text
- [x] 2.6 Create `cmd/help/version.md` and `cmd/help/completion.md`

## 3. Cobra Commands

- [x] 3.1 Create `cmd/root.go` with root command (defaults to generate, persistent `--verbose` flag)
- [x] 3.2 Create `cmd/generate.go` with generate subcommand (`--config`, `--output` flags)
- [x] 3.3 Create `cmd/rotato.go` with rotato subcommand (`--template`, `--images`, `--output`, `--frames` flags)
- [x] 3.4 Create `cmd/rotato_frame.go` with `rotato frame` nested subcommand (`--template`, `--output`, `--dim` flags)
- [x] 3.5 Create `cmd/deliver.go` with deliver subcommand (all ASC flags + env var defaults)
- [x] 3.6 Create `cmd/deliver_list.go` with `deliver list-display-types` nested subcommand
- [x] 3.7 Create `cmd/version.go` with version subcommand
- [x] 3.8 Create `cmd/completion.go` with shell completion subcommand (bash, zsh, fish, powershell)

## 4. Migration and Cleanup

- [x] 4.1 Update `cmd/delivr/main.go` to only call `cmd.Execute()`
- [x] 4.2 Move business logic functions from old `main.go` and `deliver.go` into the new command files
- [x] 4.3 Remove old `cmd/delivr/deliver.go` (logic moved to `cmd/deliver.go`)
- [x] 4.4 Update Makefile targets if command invocations changed (`rotato-frame` → `rotato frame`)
- [x] 4.5 Run `go mod tidy` and verify `go build ./cmd/delivr` succeeds
- [x] 4.6 Update README.md CLI usage examples to use new flag syntax
