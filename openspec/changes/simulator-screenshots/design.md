## Context

The mqtt-analyzer project uses fastlane's `capture_screenshots` with:
- `devices.json`: `["iPhone 17 Pro Max", "iPhone 17 Pro", "iPad Air 11-inch (M3)"]`
- `Snapfile`: `concurrent_simulators(false)`, `override_status_bar(true)`, scheme `MQTTAnalyzer`
- `SnapshotHelper.swift`: integrated into UI tests, calls `snapshot("name")` to capture
- Shell scripts: boot simulators, `xcrun simctl status_bar override`, `xcrun simctl ui appearance`
- Two passes: dark mode then light mode, screenshots go to `screenshots/{dark,light}/`
- UI test target: `MQTTAnalyzerUITests/ScreenshotTests`

Output naming: `{DeviceName}-{ScreenshotId}.png` (e.g., `iPhone 17 Pro Max-Tree View.png`)

## Goals / Non-Goals

**Goals:**
- `delivr capture --config capture.yaml` runs the full screenshot pipeline
- Capture config YAML defines: project path, scheme, test target/class, devices, appearances, output dir
- Simulator lifecycle: boot → status bar override → appearance set → run tests → collect screenshots → shutdown
- True parallel execution across ALL device+appearance combos (e.g., 3 devices × 2 appearances = 6 parallel jobs)
- Status bar override: time 9:41, WiFi active 3 bars, cellular 4 bars, battery 100% charged
- macOS support: run UI tests on the host Mac (no simulator needed)
- Provide a `SnapshotHelper.swift` that works with delivr (no fastlane dependency)
- Progress bar with per-job status

**Non-Goals:**
- Managing Xcode installation or simulator runtimes (user must have these set up)
- Supporting Android or non-Apple platforms
- Modifying the UI test code itself (users keep their existing XCUITest tests)
- App installation or build management beyond what xcodebuild test does

## Decisions

### 1. Capture config YAML format

```yaml
project: ../src/MQTTAnalyzer.xcodeproj
scheme: MQTTAnalyzer
test_target: MQTTAnalyzerUITests/ScreenshotTests
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

### 2. Simulator management via `xcrun simctl`

For each iOS device:
1. `xcrun simctl boot <udid>` — boot the simulator
2. `xcrun simctl status_bar <udid> override --time 9:41 ...` — set status bar
3. `xcrun simctl ui <udid> appearance <dark|light>` — set appearance
4. Run `xcodebuild test` targeting this specific device
5. Collect screenshots from test output
6. `xcrun simctl shutdown <udid>` — shutdown

Device UDID lookup: `xcrun simctl list devices available --json` and match by name.

For macOS: skip simctl steps, run xcodebuild directly with `destination 'platform=macOS'`.

### 3. Screenshot collection from xcresult

`xcodebuild test` produces a `.xcresult` bundle. Screenshots taken via `XCUIScreenshotProviding` are stored as attachments. We can extract them with `xcrun xcresulttool get --path <bundle>`.

Alternatively, the SnapshotHelper can write screenshots directly to a known directory (simpler, like fastlane does). This is the preferred approach — the helper writes `{DeviceName}-{ScreenshotId}.png` to a temp directory, and delivr collects them after the test run.

### 4. SnapshotHelper.swift replacement

A minimal Swift file that:
- Reads a config file (JSON) written by delivr before the test run, containing: device name, output directory
- Provides a `snapshot(_ name: String)` function that takes a screenshot and saves it
- No fastlane dependency
- Compatible with the existing `snapshot("name")` call sites in UI tests (drop-in replacement)

The config file is placed in the simulator's caches directory (same location fastlane uses).

### 5. Parallel execution

Each device+appearance combo is an independent job. All jobs can run in parallel since each simulator is a separate process. A semaphore limits concurrency if needed (default: unlimited). Progress bar tracks completion across all jobs.

For sequential mode (`parallel: false`), jobs run one at a time.

### 6. Output directory structure

```
screenshots/
  dark/
    iPhone 17 Pro Max-Tree View.png
    iPhone 17 Pro Max-JSON Data.png
    ...
  light/
    iPhone 17 Pro Max-Tree View.png
    ...
```

Same structure as the existing fastlane workflow, ensuring compatibility with the existing delivr `generate` config.

## Risks / Trade-offs

- **macOS-only** → Expected, since this drives Xcode simulators
- **Xcode version dependency** → `xcrun simctl` and `xcodebuild` behavior varies across Xcode versions. Mitigation: target Xcode 16+ (current)
- **Simulator boot time** → Booting 6 simulators in parallel can be slow. Mitigation: simulators that are already booted are reused
- **xcresult parsing** → Apple changes the format periodically. Mitigation: use SnapshotHelper direct-to-disk approach instead of xcresult extraction
- **SnapshotHelper migration** → Users need to replace fastlane's SnapshotHelper.swift with delivr's version. Mitigation: same API (`snapshot("name")`), minimal code change
