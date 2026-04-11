## ADDED Requirements

### Requirement: Rotato command generates device templates from .rotato files
The `delivr rotato` command SHALL accept an input directory (or list of `.rotato` files) and an output directory, and generate a device template set (frame PNG + mask PNG + metadata JSON) for each `.rotato` file.

#### Scenario: Generate templates from a directory of .rotato files
- **WHEN** user runs `delivr rotato --input ./rotato-files --output ./frames`
- **THEN** for each `.rotato` file in the input directory, a template set (`{stem}.frame.png`, `{stem}.frame.mask.png`, `{stem}.frame.json`) is generated in the output directory

### Requirement: Template generation uses magenta placeholder
The command SHALL generate a magenta placeholder image, render it through the Rotato app via UI automation, and then analyze the rendered output to detect the screen region and produce the template set.

#### Scenario: Magenta placeholder workflow
- **WHEN** a `.rotato` file is processed
- **THEN** a magenta placeholder of the specified dimensions is created, rendered through Rotato, and the output is analyzed to produce frame PNG (magenta → transparent), mask PNG, and metadata JSON

### Requirement: Cached raw renders
The command SHALL cache the raw Rotato render output (`.raw.png`) so that re-running the command skips the slow UI automation step and only re-runs frame detection.

#### Scenario: Re-running with cached renders
- **WHEN** `delivr rotato` is run and a `.raw.png` cache file exists for a `.rotato` file
- **THEN** the cached raw render is used for frame detection, skipping Rotato UI automation

### Requirement: Configurable placeholder dimensions
The command SHALL accept a `--dim` flag (default `1320x2868`) to specify the magenta placeholder dimensions.

#### Scenario: Custom placeholder dimensions
- **WHEN** user runs `delivr rotato --input ./rotato-files --output ./frames --dim 2064x2752`
- **THEN** the magenta placeholder is created at 2064x2752 pixels
