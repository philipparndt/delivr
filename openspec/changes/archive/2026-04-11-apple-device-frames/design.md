## Context

The device template system uses a standard format: frame PNG (bezel with transparent screen), mask PNG (opaque where screen is), and metadata JSON (corners, rectangle flag, dimensions). Currently only `delivr rotato` generates these by rendering a magenta placeholder through Rotato and detecting the screen region.

Apple provides official device bezel PNGs at developer.apple.com/design/resources with transparent screen holes. These can be converted to delivr's template format by detecting the transparent region from the alpha channel — no magenta placeholder needed.

## Goals / Non-Goals

**Goals:**
- `delivr frames` command that converts bezel PNGs into template sets
- Automatic screen region detection from the transparency (alpha channel)
- Generate mask PNG and metadata JSON with correct rectangle coordinates
- Batch processing: convert all PNGs in a directory at once
- Works with any bezel PNG that has a transparent screen hole (not Apple-specific)

**Non-Goals:**
- Downloading Apple bezels automatically (Apple doesn't provide a stable API; users download manually)
- Supporting PSD or Sketch files (only PNG)
- Perspective/angled device frames (Apple's official bezels are front-facing, always rectangular)
- Modifying the existing template format or rendering pipeline

## Decisions

### 1. New `delivr frames` command (not a subcommand of `rotato`)

The `frames` command is a peer of `rotato` — both generate template sets but from different sources. `rotato` takes `.rotato` files and uses UI automation; `frames` takes bezel PNGs and analyzes transparency.

```bash
delivr frames --input <dir-with-bezels> --output <templates-dir> [--verbose]
```

Alternative: nest under `rotato` as `rotato import`. Rejected — these aren't Rotato-related at all.

### 2. Screen detection via alpha channel analysis

Algorithm:
1. Load the bezel PNG
2. Scan all pixels to find the largest fully-transparent rectangular region (alpha == 0)
3. Use a flood-fill or bounding-box approach from the center of the image
4. The detected rectangle becomes the screen region in the metadata JSON
5. Generate the mask: opaque (white) where screen is, transparent elsewhere
6. The frame PNG is the bezel itself (unchanged — it already has the transparent hole)

For Apple's front-facing bezels, the screen is always axis-aligned (rectangular), so `IsRectangle` will be true and `RectangleRect` will be set.

### 3. Reuse existing `SaveFrame` and template format

The output uses the exact same `FrameMetadata` struct and `SaveFrame` function from `internal/rotato/frame.go`. The `FrameSource` field will have empty `RotatoTemplate` and instead record the source bezel filename.

### 4. Detection logic in a new `internal/frames/` package

Keep frame-from-bezel detection separate from the Rotato-specific magenta detection in `internal/rotato/`. The new `internal/frames/` package provides a `DetectScreenRegion(img)` function and a `GenerateTemplateFromBezel(bezelPath, outputDir)` function.

## Risks / Trade-offs

- **Imperfect detection** → Some bezels may have semi-transparent pixels at screen edges (anti-aliasing). Mitigation: use a configurable alpha threshold (default 0) to determine "fully transparent".
- **Non-rectangular screens** → Some device images may have rounded corners. The rectangle detection gives the bounding box; the mask clips at the actual transparency boundary, so rounded corners are preserved naturally.
- **Apple licensing** → Apple's design resources have usage guidelines. This tool just converts PNGs; users are responsible for compliance. Not a code concern.
