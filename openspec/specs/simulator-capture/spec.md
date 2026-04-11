## ADDED Requirements

### Requirement: Capture command runs UI tests and collects screenshots
The CLI SHALL provide a `capture` command that accepts `--config` (required, path to capture YAML) and `--verbose` flags. It SHALL boot simulators, configure them, run UI tests, collect screenshots, and organize them by appearance.

#### Scenario: Capture screenshots for all devices and appearances
- **WHEN** user runs `delivr capture --config capture.yaml`
- **THEN** UI tests are run for each device+appearance combination and screenshots are saved to the configured output directory organized by appearance subdirectories

### Requirement: Capture config YAML
The capture config SHALL define: `project` (Xcode project path), `scheme`, `test_target` (test bundle/class to run), `output` (output directory), `devices` (list with name and platform), `appearances` (list of dark/light), `status_bar` (time, wifi, cellular, battery settings), and `parallel` (boolean).

#### Scenario: Config with multiple devices and appearances
- **WHEN** config defines 3 iOS devices and 2 appearances with `parallel: true`
- **THEN** 6 capture jobs are created and executed in parallel

### Requirement: Simulator status bar override
The command SHALL override the simulator status bar for each iOS device using `xcrun simctl status_bar` with configurable time, WiFi bars, cellular bars, battery level, and battery state.

#### Scenario: Status bar set to 9:41 with full connectivity
- **WHEN** `status_bar.time` is "9:41" and `battery_level` is 100
- **THEN** the simulator status bar shows 9:41, full WiFi, full cellular, and 100% battery

### Requirement: Simulator appearance control
The command SHALL set the simulator appearance (dark or light) using `xcrun simctl ui <udid> appearance <mode>` before running tests.

#### Scenario: Dark mode screenshots
- **WHEN** appearance is "dark"
- **THEN** the simulator UI renders in dark mode for all screenshots

### Requirement: Parallel execution across all devices
When `parallel: true`, the command SHALL run all device+appearance combinations simultaneously, each on its own simulator instance. A progress bar SHALL show overall completion.

#### Scenario: Parallel capture of 6 jobs
- **WHEN** 3 devices × 2 appearances with `parallel: true`
- **THEN** all 6 xcodebuild test runs execute concurrently with a progress bar

### Requirement: Sequential execution mode
When `parallel: false`, the command SHALL run device+appearance combinations one at a time.

#### Scenario: Sequential capture
- **WHEN** `parallel: false`
- **THEN** each device+appearance job runs after the previous one completes

### Requirement: macOS screenshot support
The command SHALL support macOS as a platform. For macOS devices, the command SHALL skip simulator management and run `xcodebuild test` with `destination 'platform=macOS'` directly.

#### Scenario: macOS UI test screenshots
- **WHEN** a device has `platform: macos`
- **THEN** xcodebuild runs on the host Mac without simulator boot/shutdown and appearance is set via system preferences

### Requirement: SnapshotHelper.swift for delivr
The project SHALL include a `SnapshotHelper.swift` file that provides a `snapshot(_ name: String)` function compatible with existing UI test code. It SHALL read a delivr config file to determine the device name and output directory, and save screenshots as `{DeviceName}-{ScreenshotId}.png`.

#### Scenario: Drop-in replacement for fastlane SnapshotHelper
- **WHEN** user replaces fastlane's SnapshotHelper.swift with delivr's version
- **THEN** existing `snapshot("Tree View")` calls continue to work and save screenshots to the delivr output directory

### Requirement: Output organized by appearance
Screenshots SHALL be saved in appearance-specific subdirectories: `{output}/{appearance}/{DeviceName}-{ScreenshotId}.png`.

#### Scenario: Dark and light output structure
- **WHEN** capture completes for dark and light appearances
- **THEN** screenshots exist at `screenshots/dark/iPhone 17 Pro Max-Tree View.png` and `screenshots/light/iPhone 17 Pro Max-Tree View.png`
