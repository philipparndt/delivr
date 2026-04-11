## ADDED Requirements

### Requirement: Frames command converts bezel PNGs to template sets
The CLI SHALL provide a `frames` command that accepts `--input` (required, directory of bezel PNGs), `--output` (required, template output directory), and `--verbose` flags. It SHALL generate a template set (frame PNG + mask PNG + metadata JSON) for each PNG in the input directory.

#### Scenario: Generate templates from bezel PNGs
- **WHEN** user runs `delivr frames --input ./bezels --output ./frames`
- **THEN** for each PNG file in the input directory, a template set (`{stem}.frame.png`, `{stem}.frame.mask.png`, `{stem}.frame.json`) is generated in the output directory

### Requirement: Automatic screen region detection from transparency
The command SHALL detect the screen region by analyzing the alpha channel of the bezel PNG. The largest contiguous fully-transparent area SHALL be identified as the screen region.

#### Scenario: Detect rectangular screen from transparent hole
- **WHEN** a bezel PNG has a rectangular transparent region in the center
- **THEN** the screen region is detected with correct [x, y, width, height] coordinates and `is_rectangle` is set to true in the metadata JSON

### Requirement: Mask generation from alpha channel
The command SHALL generate a mask PNG where pixels are opaque (white) where the bezel PNG is fully transparent (screen area) and transparent elsewhere.

#### Scenario: Mask matches screen transparency
- **WHEN** a bezel PNG has a transparent screen hole with anti-aliased edges
- **THEN** the mask is opaque where the bezel is transparent and preserves the anti-aliased edge gradient

### Requirement: Template output matches existing format
The generated template sets SHALL use the same `FrameMetadata` JSON format as Rotato-generated templates, including `frame_path`, `mask_path`, `frame_width`, `frame_height`, `corners`, `is_rectangle`, and `rectangle_rect` fields.

#### Scenario: Template is compatible with renderer
- **WHEN** a template set is generated from a bezel PNG
- **THEN** it can be referenced via the `template` field in a screenshot config and renders correctly with `delivr generate`

### Requirement: Batch processing of directory
The command SHALL process all PNG files in the input directory, skipping non-PNG files.

#### Scenario: Directory with mixed files
- **WHEN** the input directory contains both PNG and non-PNG files
- **THEN** only PNG files are processed and non-PNG files are skipped with no error
