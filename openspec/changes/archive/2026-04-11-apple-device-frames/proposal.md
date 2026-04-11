## Why

The device template system currently only supports templates generated from Rotato `.rotato` files via the `delivr rotato` command. Apple provides official device bezel PNGs (with transparent screen holes) at developer.apple.com/design/resources — these are the canonical marketing frames for App Store screenshots. Adding a command to convert Apple's bezel PNGs into delivr's template format (frame + mask + JSON) enables flat device frames without requiring Rotato, and gives users access to Apple's official device imagery.

## What Changes

- Add a `delivr frames` command that converts device bezel PNGs (with transparent screen regions) into delivr template sets (frame PNG + mask PNG + metadata JSON)
- The command detects the transparent screen region automatically by analyzing the alpha channel
- Generates the screen mask and computes the rectangular screen coordinates
- Supports batch processing: point it at a directory of bezel PNGs and it generates templates for all of them
- Works with any bezel PNG that has a transparent screen hole — not limited to Apple's bezels

## Capabilities

### New Capabilities
- `frame-import`: Command to convert device bezel PNGs into delivr template sets by detecting the transparent screen region

### Modified Capabilities

## Impact

- **Code**: New `cmd/delivr/cmd/frames.go` command, new detection logic in `internal/rotato/` (or a new `internal/frames/` package)
- **Dependencies**: No new dependencies — uses existing image processing libraries
- **CLI**: Adds `delivr frames` subcommand
- **Users**: Can now use Apple's official device bezels (or any bezel PNG) as device templates alongside Rotato-generated templates
