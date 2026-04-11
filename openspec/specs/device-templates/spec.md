## ADDED Requirements

### Requirement: Renderer uses template JSON as the only device rendering path
The renderer SHALL load a device template from a `.frame.json` file when the `template` field is set on a device entry. The template JSON contains the frame PNG path, mask PNG path, corner coordinates, and rectangle detection flag.

#### Scenario: Rendering with a device template
- **WHEN** a device config entry has `template: "frames/iphone-front.frame.json"`
- **THEN** the renderer loads the frame PNG, mask PNG, and metadata from the JSON, composites the screenshot using perspective warp or rectangle blit, and applies auto-crop/scaling/positioning

### Requirement: Flat rendering when no template is specified
When a device config entry has no `template` field, the renderer SHALL load the screenshot directly as a flat image (equivalent to the old `image` mode).

#### Scenario: Flat screenshot without device frame
- **WHEN** a device config entry has no `template` field
- **THEN** the renderer loads the source screenshot PNG and applies scaling/positioning without any device frame compositing

### Requirement: Config format removes mode and mode-specific fields
The device config SHALL no longer include `mode`, `rotato_file`, or `template_rect` fields. The `template` field (path to `.frame.json`) replaces them. Screen region coordinates are stored in the template JSON, not in the config.

#### Scenario: Simplified device config
- **WHEN** a device config specifies `template: "frames/iphone-front.frame.json"` and `source: "{device}-Tree View.png"`
- **THEN** the renderer uses the template's embedded screen region metadata and does not require `mode` or `template_rect` in config

### Requirement: Template set format
A device template set SHALL consist of three files: a frame PNG (transparent where screen is), a mask PNG (opaque where screen is), and a metadata JSON file. The JSON SHALL contain paths to the frame and mask PNGs (relative to the JSON file), canvas dimensions, corner coordinates, and a rectangle detection flag.

#### Scenario: Template set files
- **WHEN** a template set exists at `frames/iphone-front.frame.json`
- **THEN** the JSON references `iphone-front.frame.png` and `iphone-front.frame.mask.png` in the same directory, and contains `corners`, `isRectangle`, `rectangleRect`, `frameWidth`, and `frameHeight` fields
