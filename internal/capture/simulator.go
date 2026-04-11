package capture

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// SimDevice represents a simulator device from simctl.
type SimDevice struct {
	UDID      string `json:"udid"`
	Name      string `json:"name"`
	State     string `json:"state"`
	IsAvailable bool `json:"isAvailable"`
	Runtime   string // runtime identifier, e.g. "com.apple.CoreSimulator.SimRuntime.iOS-26-4"
}

type simctlOutput struct {
	Devices map[string][]SimDevice `json:"devices"`
}

// ListDevices returns all available simulator devices with their runtime info.
func ListDevices() ([]SimDevice, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list simulators: %w", err)
	}

	var result simctlOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse simctl output: %w", err)
	}

	var devices []SimDevice
	for runtime, devs := range result.Devices {
		for _, d := range devs {
			d.Runtime = runtime
			devices = append(devices, d)
		}
	}
	return devices, nil
}

// FindDeviceResult holds the UDID and runtime of a found simulator.
type FindDeviceResult struct {
	UDID    string
	Runtime string
}

// FindDevice finds a simulator device by name, returning the UDID and runtime
// of the one with the latest (highest) runtime version.
func FindDevice(name string) (FindDeviceResult, error) {
	devices, err := ListDevices()
	if err != nil {
		return FindDeviceResult{}, err
	}

	var matches []SimDevice
	for _, d := range devices {
		if d.Name == name {
			matches = append(matches, d)
		}
	}

	if len(matches) == 0 {
		return FindDeviceResult{}, fmt.Errorf("simulator not found: %q", name)
	}

	// Sort by runtime version descending — pick the latest
	sort.Slice(matches, func(i, j int) bool {
		return runtimeSortKey(matches[i].Runtime) > runtimeSortKey(matches[j].Runtime)
	})

	return FindDeviceResult{
		UDID:    matches[0].UDID,
		Runtime: RuntimeDisplayName(matches[0].Runtime),
	}, nil
}

// RuntimeDisplayName converts a runtime identifier to a human-readable name.
// e.g. "com.apple.CoreSimulator.SimRuntime.iOS-26-4" → "iOS 26.4"
func RuntimeDisplayName(runtime string) string {
	parts := strings.SplitN(runtime, "SimRuntime.", 2)
	if len(parts) < 2 {
		return runtime
	}
	name := parts[1] // e.g. "iOS-26-4"
	// Replace first "-" with " " (platform separator), rest with "."
	segments := strings.Split(name, "-")
	if len(segments) >= 2 {
		return segments[0] + " " + strings.Join(segments[1:], ".")
	}
	return name
}

// runtimeSortKey extracts a sortable string from a runtime identifier.
// e.g. "com.apple.CoreSimulator.SimRuntime.iOS-26-4" → "iOS-026-004"
func runtimeSortKey(runtime string) string {
	// Extract the part after "SimRuntime."
	parts := strings.SplitN(runtime, "SimRuntime.", 2)
	if len(parts) < 2 {
		return runtime
	}
	key := parts[1] // e.g. "iOS-26-4"

	// Split by "-" and zero-pad numeric parts for proper sorting
	segments := strings.Split(key, "-")
	for i, seg := range segments {
		if n, err := strconv.Atoi(seg); err == nil {
			segments[i] = fmt.Sprintf("%03d", n)
		}
	}
	return strings.Join(segments, "-")
}

// BootDevice boots a simulator. If already booted, this is a no-op.
func BootDevice(udid string) error {
	err := exec.Command("xcrun", "simctl", "boot", udid).Run()
	if err != nil {
		// Already booted or other non-fatal error — continue
		return nil
	}
	return nil
}

// ShutdownDevice shuts down a simulator.
func ShutdownDevice(udid string) error {
	return exec.Command("xcrun", "simctl", "shutdown", udid).Run()
}

// WaitForBoot waits for a simulator to finish booting.
func WaitForBoot(udid string) error {
	return exec.Command("xcrun", "simctl", "bootstatus", udid, "-b").Run()
}

// SetStatusBar overrides the simulator status bar.
func SetStatusBar(udid string, cfg StatusBarConfig) error {
	args := []string{
		"simctl", "status_bar", udid, "override",
		"--time", cfg.Time,
		"--dataNetwork", "wifi",
		"--wifiMode", "active",
		"--wifiBars", strconv.Itoa(cfg.WifiBars),
		"--cellularMode", "active",
		"--cellularBars", strconv.Itoa(cfg.CellularBars),
		"--batteryState", cfg.BatteryState,
		"--batteryLevel", strconv.Itoa(cfg.BatteryLevel),
	}
	return exec.Command("xcrun", args...).Run()
}

// SetAppearance sets the simulator appearance (dark or light).
func SetAppearance(udid, mode string) error {
	return exec.Command("xcrun", "simctl", "ui", udid, "appearance", mode).Run()
}
