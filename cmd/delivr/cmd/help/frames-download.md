Download Apple device bezels from Apple's official CDN.

Downloads `.dmg` files from `devimages-cdn.apple.com`, mounts them,
extracts PNG bezel images, and places them in the output directory.
The extracted PNGs are ready for `delivr frames --input`.

Requires macOS (uses `hdiutil` for DMG extraction).

## Usage

```bash
delivr frames download --output <dir> [--device <slug>] [--verbose]
```

## Examples

```bash
# Download all available device bezels
delivr frames download --output ./bezels

# Download only iPhone 17 bezels
delivr frames download --output ./bezels --device iphone-17

# List all available devices
delivr frames download --list

# Download from a custom DMG URL
delivr frames download --output ./bezels --url https://example.com/Bezel-Custom.dmg
```

## Workflow

```bash
# 1. Download bezels
delivr frames download --output ./bezels

# 2. Convert to device templates
delivr frames --input ./bezels --output ./frames

# 3. Use in screenshot config
delivr generate --config config.yaml
```

Re-running the command skips already-downloaded devices.
