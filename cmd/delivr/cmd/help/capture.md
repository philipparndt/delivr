Capture screenshots from iOS simulators and macOS by running UI tests.

Replaces fastlane's `capture_screenshots` with a single command that
boots simulators, sets status bars, toggles dark/light mode, runs
your XCUITests, and collects the resulting screenshots.

## Workflow

```bash
# 1. Create a capture config (see example below)
# 2. Replace fastlane's SnapshotHelper.swift with delivr's version
#    (copy from delivr's snapshot/ directory to your UI test target)
# 3. Run capture
delivr capture --config capture.yaml

# 4. Generate App Store screenshots from captured images
delivr generate --config screenshot-config.yaml
```

## Usage

```bash
delivr capture --config <capture.yaml> [--verbose]
```

## Capture Config Example

```yaml
project: ../src/MyApp.xcodeproj
scheme: MyApp
test_target: MyAppUITests/ScreenshotTests
output: ./screenshots

devices:
  - name: "iPhone 17 Pro Max"
    platform: ios
  - name: "iPhone 17 Pro"
    platform: ios
  - name: "iPad Air 11-inch (M3)"
    platform: ios
  - name: "My Mac"
    platform: macos

appearances:
  - dark
  - light

status_bar:
  time: "9:41"
  wifi_bars: 3
  cellular_bars: 4
  battery_level: 100
  battery_state: charged

parallel: true
```

## Features

- **Parallel execution** across all device × appearance combinations
- **Status bar override**: time, WiFi, cellular, battery
- **Dark/light mode** per screenshot set
- **iOS + macOS** support
- **Progress bar** showing capture progress
- **Drop-in SnapshotHelper.swift** — same `snapshot("name")` API as fastlane
