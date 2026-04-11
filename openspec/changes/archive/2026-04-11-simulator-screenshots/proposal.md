## Why

The current screenshot workflow uses fastlane's `capture_screenshots` (via SnapshotHelper.swift) plus shell scripts to boot simulators, set status bars, toggle dark/light mode, and run UI tests. This has several limitations: parallel execution is only per device group (not across all devices), macOS screenshots aren't supported, and the setup requires Ruby, fastlane gems, and multiple shell scripts. By adding a `delivr capture` command that directly drives `xcrun simctl` and `xcodebuild`, delivr can replace fastlane entirely for screenshot generation — with true cross-device parallelism, dark/light theme support, status bar overrides, and both iOS and macOS platform support.

## What Changes

- Add `delivr capture` command that runs UI tests on iOS simulators and macOS to capture screenshots
- Configuration via YAML (capture config) defining: Xcode project/scheme, devices, appearances, output directory
- Simulator management: boot, set status bar (9:41, full battery, wifi), set appearance (dark/light), shutdown
- Run `xcodebuild test` with the UI test scheme per device+appearance combination
- Collect screenshots from the test results (`.xcresult` bundle) or simulator screenshot directory
- Support parallel execution across ALL device+appearance combinations simultaneously
- Support macOS screenshots by running UI tests on the host Mac directly
- Include a delivr-native SnapshotHelper.swift replacement that saves screenshots without fastlane dependency
- Progress bar showing capture progress across all device+appearance jobs

## Capabilities

### New Capabilities
- `simulator-capture`: Command to capture screenshots via iOS simulators and macOS UI tests with status bar override, appearance control, and parallel execution

### Modified Capabilities

## Impact

- **Code**: New `cmd/delivr/cmd/capture.go`, new `internal/capture/` package with simulator management, xcodebuild runner, and result extraction
- **Dependencies**: No new Go dependencies — uses `os/exec` for `xcrun simctl` and `xcodebuild`
- **Platform**: macOS only (requires Xcode and simulators)
- **CLI**: Adds `delivr capture` subcommand
- **Users**: Can replace fastlane snapshot + shell scripts with a single `delivr capture` command and YAML config
