## Why

The `delivr frames` command can convert bezel PNGs into device templates, but users must manually download Apple's bezels from developer.apple.com/design/resources. Apple provides these as `.dmg` files from a CDN with predictable URLs (e.g., `https://devimages-cdn.apple.com/design/resources/download/Bezel-iPhone-17.dmg`). A `delivr frames download` subcommand can automate this: download the DMG, mount it, extract the PNG files, unmount, and place them ready for `delivr frames --input`.

Note: The third-party `extratone/bezels` GitHub repo only has devices up to iPhone 13 and appears unmaintained. Apple's official CDN has current devices (iPhone 17, iPhone 16, iPad Pro M4, etc.).

## What Changes

- Add `delivr frames download` subcommand that downloads Apple device bezel DMGs from the official CDN
- Ships with a built-in list of known device bezel URLs (iPhone 17, iPhone 16, iPad Pro M4, iPad Air M2, iPad mini, etc.)
- Downloads each DMG, mounts it (macOS `hdiutil`), copies PNG files to the output directory, and unmounts
- Users can also specify a `--device` flag to download only specific devices
- Users can specify a `--url` flag to download from a custom DMG URL not in the built-in list

## Capabilities

### New Capabilities
- `frames-download`: Subcommand to download and extract Apple device bezel PNGs from Apple's CDN

### Modified Capabilities

## Impact

- **Code**: New `cmd/delivr/cmd/frames_download.go` as subcommand of `frames`, new download/extract logic
- **Dependencies**: No new Go dependencies — uses `net/http` for download and `os/exec` for `hdiutil` (macOS)
- **CLI**: Adds `delivr frames download` subcommand
- **Platform**: DMG mounting requires macOS; the download itself works cross-platform but extraction is macOS-only
