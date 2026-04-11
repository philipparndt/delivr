Generate device templates from bezel PNG images.

Converts device bezel PNGs (with transparent screen holes) into delivr
template sets (frame PNG + mask PNG + metadata JSON). Works with Apple's
official device bezels or any bezel PNG with a transparent screen region.

## Workflow

```bash
# 1. Download Apple device bezels (macOS)
delivr frames download --output ./bezels

# 2. Convert bezels to device templates
delivr frames --input ./bezels --output ./frames

# 3. Reference templates in your screenshot config
#    device:
#      source: "{device}-Tree View.png"
#      template: "frames/iPhone 17 Pro Max - Silver - Portrait.frame.json"
#      height: 2000

# 4. Generate screenshots
delivr generate --config config.yaml --output ./output
```

## Usage

```bash
delivr frames --input <dir-with-bezels> --output <templates-dir> [--verbose]
```

## How It Works

1. Scans the input directory for PNG files
2. Detects the transparent screen region via alpha channel flood-fill
3. Generates a screen mask (opaque where screen is, with rounded corners)
4. Saves a template set: `{name}.frame.png`, `{name}.frame.mask.png`, `{name}.frame.json`

## Subcommands

| Command    | Description                                    |
|------------|------------------------------------------------|
| `download` | Download Apple device bezels from Apple's CDN  |
