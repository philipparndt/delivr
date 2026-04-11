## 1. YAML Include Loader

- [x] 1.1 Create `internal/config/include.go` with `!include` tag processor using `yaml.Node` tree walking: resolve includes recursively, flatten sequences, detect cycles
- [x] 1.2 Create `internal/config/include_test.go` with tests for basic include, nested include, sequence flattening, and cycle detection
- [x] 1.3 Update `internal/config/loader.go` to use the include-aware loader before unmarshaling

## 2. Unified Config Types

- [x] 2.1 Create `internal/config/root.go` with `RootConfig` struct: `Settings`, `Devices`, `Capture`, `Generate`, `Deliver` sections
- [x] 2.2 Add `platform` field to device config types so devices are shared between capture and generate
- [x] 2.3 Add auto-detection logic: if config has `capture`/`generate`/`deliver` keys, parse as root config; otherwise parse as standalone

## 3. Command Updates

- [x] 3.1 Update `delivr generate` to accept root config and extract `generate` + `devices` + `settings` sections
- [x] 3.2 Update `delivr capture` to accept root config and extract `capture` + `devices` sections
- [x] 3.3 Update `delivr deliver` to accept root config and extract `deliver` + `devices` + `settings` sections

## 4. Init Command

- [x] 4.1 Create `cmd/delivr/cmd/init.go` with `delivr init` command that prompts for device selection (from a built-in list of common Apple devices)
- [x] 4.2 Generate scaffold files: `delivr.yaml`, `devices.yaml`, `templates.yaml`, `screens/example.yaml`, `outputs.yaml`
- [x] 4.3 Generate example App Store metadata files: `metadata/en-US/description.md`, `metadata/en-US/promotional_text.md`, `metadata/en-US/release_notes.md`
- [x] 4.4 Generate `.claude/commands/changelog.md` (generate changelog from git log), `.claude/commands/update-description.md` (update App Store description), `.claude/commands/translate.md` (translate metadata to other languages)
- [x] 4.5 Create `cmd/delivr/cmd/help/init.md` with command help and add embed to `help.go`

## 5. Build and Documentation

- [x] 5.1 Run `go build ./cmd/delivr` and verify
- [x] 5.2 Update README.md with unified config documentation and `delivr init` workflow
