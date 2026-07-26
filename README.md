# delivr

A lightweight App Store screenshot generator and delivery tool — a fastlane alternative for screenshot automation.

## Features

- Capture screenshots from iOS simulators and macOS with parallel execution
- Generate professional App Store screenshots with device frames, backgrounds, and text
- Visual editor (`delivr edit`) with live preview, geometry overlays and solved positioning
- 3D device mockups via [Rotato](https://rotato.app) integration (macOS)
- Download and import Apple's official device bezels
- Upload screenshots and metadata to App Store Connect
- Multi-language/localization support
- Unified `delivr.yaml` config with `!include` support for modular organization
- Claude Code commands for changelog, description, and translation workflows
- Shell completion (bash, zsh, fish, powershell)

## Installation

### Homebrew

```bash
brew install philipparndt/delivr/delivr
```

### Binary Download

Download the latest release from [GitHub Releases](https://github.com/philipparndt/delivr/releases).

### Build from Source

```bash
git clone https://github.com/philipparndt/delivr.git
cd delivr
make build
```

## Quick Start

```bash
# Initialize a new project (interactive device selection)
delivr init

# Set up your SnapshotHelper
delivr capture init -o path/to/MyUITests/SnapshotHelper.swift

# Capture screenshots from simulators
delivr capture --config delivr.yaml

# Generate App Store screenshots
delivr generate --config delivr.yaml

# Deliver to App Store Connect
delivr deliver --config delivr.yaml
```

## Configuration

delivr uses a unified `delivr.yaml` root config with `!include` support for modular organization:

```yaml
# delivr.yaml
settings:
  fonts_dir: "/Library/Fonts"
  bundle_id: "com.example.myapp"

devices: !include devices.yaml

capture:
  project: ../src/MyApp.xcodeproj
  scheme: MyApp
  test_target: MyAppUITests/ScreenshotTests
  appearances: [dark, light]
  parallel: true

generate:
  templates: !include templates.yaml
  screens:
    - !include screens/iphone.yaml
    - !include screens/ipad.yaml
  outputs: !include outputs.yaml

deliver:
  metadata_dir: ./metadata
  screenshots_dir: ./output/appstore
```

Run `delivr init` to generate a complete project scaffold interactively.

## Commands

### `delivr init`

Initialize a new project with example configs, metadata templates, and Claude Code commands.

### `delivr capture`

Capture screenshots from iOS simulators and macOS with status bar override, dark/light mode, and parallel execution.

```bash
delivr capture --config delivr.yaml
```

### `delivr generate`

Compose captured screenshots into App Store images with device frames, backgrounds, and text.

```bash
delivr generate --config delivr.yaml
```

### `delivr edit`

Position devices and copy against a live preview, rendered by the same code
`generate` writes files with.

```bash
delivr edit --config delivr.yaml
```

Drag a device to set `x`/`y`, size it with the height slider, and edit the copy in place,
with ⌘Z/⇧⌘Z to undo and redo. The preview is sent as cached layers, so dragging
is a CSS offset of the renderer's own pixels rather than a round trip. Overlays show what the numbers do not: the
`auto_crop` box against the device body (they do not share a centre, which is
why `x: 0` is not centred), the margin to each canvas edge, and which anchoring
mode each text block is in. "Centre it", "bleed to bottom" and "fit" solve for
values instead of nudging them.

The **whole store row** renders at once, in order, with the App Store's
carousel gap between screenshots (~4% of the width), scrolling horizontally.
Every screen is live: click any screenshot or device to select it, and drag the
strip beneath a screenshot to reorder the row. Every
boundary with a device spanning it is checked on `x`, `y` and `height`, so a
pair meant to read as one object can be lined up by number instead of by eye.

A **jointed ruler** spans the row at any angle. Segments chain, and Shift snaps
each one in 15° steps *relative to its predecessor* — which is how you check
"perpendicular to that phone's edge" when the edge is itself at 17°.

Most geometry lives in a shared template, so the panel names the template
driving each screen and how many others move with it. **Add** copies a device
from another screen onto this one, placed at the seam-continuation `x` when the
two are neighbours.

Nothing is written until you press Save (⌘S), and each value goes to a file you
choose — the shared template, this screen, or the translation file for
localised copy — as a single-token edit that leaves comments, blank lines and
quoting untouched. `--read-only` disables writing entirely.

### `delivr frames`

Generate device templates from bezel PNGs or download Apple's official bezels.

```bash
# Download Apple bezels
delivr frames download --output ./bezels --device iphone-17

# Convert to templates
delivr frames --input ./bezels --output ./frames

# Generate from Rotato files
delivr rotato --input ./rotato-files --output ./frames
```

### `delivr deliver`

Upload screenshots and metadata to App Store Connect.

```bash
delivr deliver --config delivr.yaml
```

### `delivr capture init`

Generate the SnapshotHelper.swift for your UI test target.

```bash
delivr capture init -o path/to/MyUITests/SnapshotHelper.swift
```

## Example Configs

- `configs/example.yaml` — Basic flat screenshots
- `configs/example-rotato.yaml` — Device templates with 3D Rotato mockups
- `configs/example-capture.yaml` — Simulator screenshot capture

## License

Apache-2.0
