Generate device template sets from Rotato `.rotato` files.

Each `.rotato` file is rendered with a magenta placeholder through the
Rotato macOS app. The output is analyzed to detect the device screen
region and saved as a template set: frame PNG, mask PNG, and metadata JSON.

These template sets are then used by `delivr generate` to composite
screenshots into 3D device mockups.

## Usage

```bash
delivr rotato --input <dir-with-rotato-files> --output <templates-dir> [--dim WxH]
```

## Workflow

1. Place your `.rotato` scene files in a directory
2. Run `delivr rotato` to generate template sets
3. Reference the `.frame.json` files in your screenshot config
4. Run `delivr generate` to produce final screenshots

## Caching

Raw Rotato renders are cached as `.raw.png` files. Re-running the
command skips the slow UI automation and only re-runs frame detection.
Delete the `.raw.png` files to force re-rendering.

## Examples

```bash
# Generate templates for all .rotato files
delivr rotato --input ./rotato-files --output ./frames

# Use custom placeholder dimensions (e.g., for iPad)
delivr rotato --input ./rotato-files --output ./frames --dim 2064x2752
```
