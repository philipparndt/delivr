## 1. Download and Extract Logic

- [x] 1.1 Create `internal/frames/catalog.go` with the built-in device catalog map (slug → CDN URL) covering current Apple devices
- [x] 1.2 Create `internal/frames/download.go` with `DownloadDMG()` (HTTP download to temp file), `ExtractPNGs()` (hdiutil mount, copy PNGs, unmount), and `DownloadAndExtract()` orchestrator
- [x] 1.3 Add skip-existing logic: check if PNGs for a device slug already exist in the output directory before downloading

## 2. CLI Command

- [x] 2.1 Create `cmd/delivr/cmd/frames_download.go` with `delivr frames download` subcommand (`--output`, `--device`, `--url`, `--list`, `--verbose` flags)
- [x] 2.2 Create `cmd/delivr/cmd/help/frames-download.md` with command help text
- [x] 2.3 Add `frames-download` embed and case to `cmd/delivr/cmd/help.go`

## 3. Build and Documentation

- [x] 3.1 Run `go build ./cmd/delivr` and verify the command works
- [x] 3.2 Update README.md to document the `delivr frames download` workflow
