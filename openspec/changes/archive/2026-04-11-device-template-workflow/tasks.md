## 1. Config and Types Cleanup

- [x] 1.1 Update `internal/config/types.go`: remove `Mode`, `RoratoFile`, `TemplateRect` fields from `DeviceImage`; keep `Template` (now points to `.frame.json`), `Source`, positioning/sizing fields, and `AutoCrop` fields
- [x] 1.2 Update `internal/config/loader.go` if it validates or defaults the `mode` field
- [x] 1.3 Update example config files (`configs/example.yaml`, `configs/example-rotato.yaml`) to use new format

## 2. Renderer Simplification

- [x] 2.1 Rewrite `internal/render/device.go` `RenderDevice()`: remove mode switch; if `Template` is set, load template JSON and call `RenderWithFrame()`; if not, load source image directly (flat mode)
- [x] 2.2 Remove any imports/references to `rotato.RenderWithCLI` and `rotato.RenderWithTemplate` from the render package

## 3. Rework Rotato CLI Command

- [x] 3.1 Rewrite `cmd/delivr/cmd/rotato.go`: change flags from `--template/--images/--output/--frames` to `--input/--output/--dim/--verbose`; iterate over `.rotato` files in input dir and generate template sets for each
- [x] 3.2 Remove `cmd/delivr/cmd/rotato_frame.go` (functionality absorbed into reworked `rotato.go`)
- [x] 3.3 Update `cmd/delivr/cmd/help/rotato.md` with new command usage
- [x] 3.4 Remove `cmd/delivr/cmd/help/rotato-frame.md` and its embed in `help.go`

## 4. Cleanup Unused Code

- [x] 4.1 Remove `RenderWithTemplate()` from `internal/rotato/rotato.go` if it exists as a separate function from `RenderWithFrame()`
- [x] 4.2 Verify `RenderWithCLI()` is only called from the rotato command, not from the render package
- [x] 4.3 Run `go mod tidy` and verify `go build ./cmd/delivr` succeeds

## 5. Documentation

- [x] 5.1 Update README.md to reflect new workflow (generate templates first, then render screenshots)
- [x] 5.2 Update Makefile rotato targets to use new command syntax
