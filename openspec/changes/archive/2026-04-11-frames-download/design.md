## Context

Apple provides device bezels at `https://devimages-cdn.apple.com/design/resources/download/Bezel-{Name}.dmg`. Each DMG contains PSD and PNG files. On macOS, DMGs can be mounted with `hdiutil attach`, contents copied, then unmounted with `hdiutil detach`.

The `delivr frames` command already converts bezel PNGs into template sets. This change adds a download step before that conversion.

## Goals / Non-Goals

**Goals:**
- `delivr frames download --output <dir>` downloads all known Apple device bezels
- Built-in catalog of known device bezel URLs (easily updatable as new devices launch)
- Download DMG → mount → extract PNGs → unmount → place in output directory
- `--device` filter to download only specific devices (e.g., `--device iphone-17`)
- `--url` to download from a custom DMG URL
- Progress output showing which devices are being downloaded
- Skip already-downloaded files (idempotent re-runs)

**Non-Goals:**
- Cross-platform DMG extraction (macOS only — DMGs are an Apple format)
- Automatic template generation after download (user runs `delivr frames --input` separately)
- Scraping the Apple design resources page for URLs (use a static catalog)

## Decisions

### 1. Built-in device catalog as a Go map

A `var DeviceCatalog` map in the download package maps device slugs to CDN URLs:

```go
var DeviceCatalog = map[string]string{
    "iphone-17":       "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPhone-17.dmg",
    "iphone-16":       "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPhone-16.dmg",
    "ipad-pro-m4":     "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPad-Pro-M4.dmg",
    ...
}
```

When Apple launches new devices, a PR updates this map. No config files or remote catalogs needed.

### 2. DMG handling via hdiutil

```
hdiutil attach -nobrowse -quiet <dmg-path>   → mounts at /Volumes/<name>
# copy PNGs from mounted volume
hdiutil detach -quiet /Volumes/<name>
```

The `-nobrowse` flag prevents Finder from opening the volume. Error handling detects if `hdiutil` is unavailable (non-macOS) and gives a clear message.

### 3. Skip existing files

Before downloading, check if the target PNG(s) already exist in the output directory. If they do, skip the download. This makes re-runs fast and idempotent.

### 4. Subcommand of `frames`

`delivr frames download` is a subcommand of the existing `frames` command. This groups related functionality: `frames` converts bezels, `frames download` fetches them.

## Risks / Trade-offs

- **macOS only** → DMG mounting requires macOS. Mitigation: clear error message on other platforms. This is acceptable since delivr's Rotato features are also macOS-only.
- **Apple URL changes** → If Apple changes the CDN URL pattern, the catalog breaks. Mitigation: the `--url` flag provides an escape hatch; updating the catalog is a one-line code change.
- **DMG contents vary** → Different DMGs may have different internal directory structures. Mitigation: recursively search for PNG files inside the mounted volume.
