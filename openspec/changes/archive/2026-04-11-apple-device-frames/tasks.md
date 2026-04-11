## 1. Screen Detection Package

- [x] 1.1 Create `internal/frames/` package with `DetectScreenRegion()` function that finds the transparent rectangular region in a bezel PNG by analyzing the alpha channel
- [x] 1.2 Add `GenerateMask()` function that creates a mask PNG (opaque where bezel is transparent, transparent elsewhere) with anti-aliased edge support
- [x] 1.3 Add `GenerateTemplate()` function that takes a bezel PNG path and output directory, detects the screen region, generates the mask, and saves the template set using `rotato.SaveFrame()`

## 2. CLI Command

- [x] 2.1 Create `cmd/delivr/cmd/frames.go` with `delivr frames` command (`--input`, `--output`, `--verbose` flags)
- [x] 2.2 Create `cmd/delivr/cmd/help/frames.md` with command help text
- [x] 2.3 Add `frames` embed and case to `cmd/delivr/cmd/help.go`

## 3. Build and Test

- [x] 3.1 Run `go mod tidy` and verify `go build ./cmd/delivr` succeeds
- [x] 3.2 Update README.md to document the `delivr frames` command
