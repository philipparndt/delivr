## ADDED Requirements

### Requirement: Download subcommand fetches Apple device bezels
The CLI SHALL provide `delivr frames download` that accepts `--output` (required) and downloads Apple device bezel DMGs from the official CDN, extracts PNG files, and places them in the output directory.

#### Scenario: Download all known device bezels
- **WHEN** user runs `delivr frames download --output ./bezels`
- **THEN** all devices in the built-in catalog are downloaded, DMGs are mounted, PNGs are extracted to `./bezels/`, and DMGs are cleaned up

### Requirement: Device filter
The command SHALL accept a `--device` flag to download only specific devices by their catalog slug.

#### Scenario: Download specific device
- **WHEN** user runs `delivr frames download --output ./bezels --device iphone-17`
- **THEN** only the iPhone 17 bezel is downloaded and extracted

### Requirement: Custom URL support
The command SHALL accept a `--url` flag to download a bezel DMG from a URL not in the built-in catalog.

#### Scenario: Download from custom URL
- **WHEN** user runs `delivr frames download --output ./bezels --url https://example.com/Bezel-Custom.dmg`
- **THEN** the DMG is downloaded, mounted, PNGs extracted, and placed in the output directory

### Requirement: Skip existing files
The command SHALL skip downloading devices whose PNG files already exist in the output directory.

#### Scenario: Idempotent re-run
- **WHEN** user runs `delivr frames download` and the output directory already contains the iPhone 17 bezel PNGs
- **THEN** the iPhone 17 download is skipped and a message indicates it was skipped

### Requirement: Built-in device catalog
The command SHALL include a built-in catalog of known Apple device bezel URLs covering current devices (iPhone 17, iPhone 16, iPad Pro M4, iPad Air M2, iPad mini, iPad, Apple Watch, Mac, etc.). The command SHALL support `--list` to display all available devices.

#### Scenario: List available devices
- **WHEN** user runs `delivr frames download --list`
- **THEN** all device slugs and their descriptions are printed

### Requirement: macOS-only DMG extraction
The command SHALL use `hdiutil` to mount and unmount DMG files. If `hdiutil` is not available, the command SHALL fail with a clear error message indicating macOS is required.

#### Scenario: Non-macOS platform
- **WHEN** the command is run on Linux
- **THEN** an error message is displayed explaining that DMG extraction requires macOS
