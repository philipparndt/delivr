## 1. Capture Config

- [x] 1.1 Create `internal/capture/config.go` with capture config YAML types: project, scheme, test_target, output, devices (name + platform), appearances, status_bar, parallel
- [x] 1.2 Add config loader with path resolution relative to the config file

## 2. Simulator Management

- [x] 2.1 Create `internal/capture/simulator.go` with `ListDevices()` (parse `xcrun simctl list devices available --json`), `FindDevice(name)` (lookup UDID by name), `BootDevice(udid)`, `ShutdownDevice(udid)`
- [x] 2.2 Add `SetStatusBar(udid, config)` using `xcrun simctl status_bar <udid> override`
- [x] 2.3 Add `SetAppearance(udid, mode)` using `xcrun simctl ui <udid> appearance <dark|light>`

## 3. Test Runner

- [x] 3.1 Create `internal/capture/runner.go` with `RunTests(project, scheme, testTarget, deviceName, platform)` that executes `xcodebuild test` with the correct `-destination` for iOS simulator or macOS
- [x] 3.2 Add screenshot collection: after test run, move screenshots from the SnapshotHelper output directory to the final `{output}/{appearance}/` directory

## 4. SnapshotHelper

- [x] 4.1 Create `snapshot/SnapshotHelper.swift` — delivr-native replacement for fastlane's SnapshotHelper that reads a config JSON (device name, output dir) from the simulator caches and provides `snapshot(_ name: String)` function
- [x] 4.2 Add `internal/capture/helper.go` with `WriteHelperConfig(udid, deviceName, outputDir)` that writes the config JSON to the simulator's caches directory before running tests

## 5. Capture Orchestrator

- [x] 5.1 Create `internal/capture/capture.go` with `RunCapture(config)` orchestrator: generates jobs (device × appearance), manages simulator lifecycle, runs tests, collects screenshots
- [x] 5.2 Add parallel execution with semaphore, progress bar, and error collection
- [x] 5.3 Add sequential execution mode

## 6. CLI Command

- [x] 6.1 Create `cmd/delivr/cmd/capture.go` with `delivr capture --config <yaml>` command
- [x] 6.2 Create `cmd/delivr/cmd/help/capture.md` with command help text and workflow example
- [x] 6.3 Add `capture` embed and case to `cmd/delivr/cmd/help.go`

## 7. Build and Documentation

- [x] 7.1 Run `go build ./cmd/delivr` and verify
- [x] 7.2 Create an example `configs/example-capture.yaml` showing a typical iOS+macOS capture config
- [x] 7.3 Update README.md to document the full `delivr capture` workflow
