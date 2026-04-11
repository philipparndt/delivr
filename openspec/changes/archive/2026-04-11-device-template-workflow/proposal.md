## Why

The current rendering pipeline has 4 device modes (`image`, `rotato-cli`, `rotato-template`, `rotato-image`), some of which call the Rotato macOS app at render time. This couples screenshot generation to a macOS-only UI automation step, makes the pipeline slow and fragile, and prevents using flat device frames (e.g., Apple's official device bezels). By standardizing on pre-rendered device templates — a frame PNG with transparent screen region, a mask, and corner metadata JSON — the rendering pipeline becomes fast, platform-independent, and extensible to any device frame source.

## What Changes

- **Remove** `rotato-cli` and `rotato-image` device modes from the rendering pipeline
- **Remove** the `image` mode — replace with a unified template-based approach where "flat" is just a template with a rectangular screen region
- **Consolidate** to a single device mode that uses pre-rendered template files (frame PNG + mask PNG + metadata JSON)
- **Rework `delivr rotato` command**: takes a directory of `.rotato` files and generates device template sets (frame + mask + JSON) for each, replacing the current batch-screenshot-through-Rotato workflow
- **Remove `delivr rotato frame`** as a separate subcommand — its functionality becomes the core of the reworked `rotato` command
- **Simplify config**: device entries reference a template directory/JSON path instead of specifying a mode and mode-specific fields
- **BREAKING**: Removes `rotato-cli`, `rotato-image`, and `image` device modes; config format changes for device entries

## Capabilities

### New Capabilities
- `device-templates`: Unified device template system — rendering only consumes pre-rendered template files (frame + mask + JSON)
- `rotato-generate`: Reworked `delivr rotato` command that generates device templates from `.rotato` files

### Modified Capabilities
- `cobra-commands`: CLI command structure changes (rotato subcommand reworked, rotato frame removed)

## Impact

- **Code**: `internal/render/device.go` simplified to single template path; `internal/rotato/rotato.go` RenderWithCLI still used internally by the rotato command but no longer by the renderer; `cmd/delivr/cmd/rotato.go` and `rotato_frame.go` restructured
- **Config**: `device.mode` field removed; devices reference a `template` JSON path instead
- **CLI**: `delivr rotato` changes from "batch process screenshots" to "generate device templates from .rotato files"
- **Users**: Must pre-generate templates before rendering; `delivr rotato` is now a one-time setup step
