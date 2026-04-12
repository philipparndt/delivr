package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildForTesting runs `xcodebuild build-for-testing` for each platform
// separately. Mixing macOS and iOS Simulator destinations in a single
// xcodebuild invocation can cause SIGTRAP crashes in some Xcode versions.
func BuildForTesting(project, scheme string, devices []DeviceConfig, verbose bool) error {
	// Group destinations by platform to build separately
	var iosDestinations []string
	hasMacOS := false

	for _, device := range devices {
		if device.Platform == "macos" {
			hasMacOS = true
		} else {
			result, err := FindDevice(device.Name)
			if err != nil {
				continue
			}
			iosDestinations = append(iosDestinations, fmt.Sprintf("platform=iOS Simulator,id=%s", result.UDID))
		}
	}

	// Build for iOS Simulator destinations
	if len(iosDestinations) > 0 {
		if err := runBuildForTesting(project, scheme, iosDestinations, verbose); err != nil {
			return err
		}
	}

	// Build for macOS separately
	if hasMacOS {
		if err := runBuildForTesting(project, scheme, []string{"platform=macOS,arch=arm64"}, verbose); err != nil {
			return err
		}
	}

	return nil
}

func runBuildForTesting(project, scheme string, destinations []string, verbose bool) error {
	args := []string{
		"build-for-testing",
		"-project", project,
		"-scheme", scheme,
	}
	for _, dest := range destinations {
		args = append(args, "-destination", dest)
	}

	cmd := exec.Command("xcodebuild", args...)

	var stderr bytes.Buffer
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			lines := strings.Split(strings.TrimSpace(errMsg), "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			return fmt.Errorf("build-for-testing failed: %w\n%s", err, strings.Join(lines, "\n"))
		}
		return fmt.Errorf("build-for-testing failed: %w", err)
	}

	return nil
}

// RunTests executes xcodebuild test-without-building for a specific device.
func RunTests(project, scheme, testTarget, deviceName, platform, udid string, verbose bool) error {
	var destination string
	if platform == "macos" {
		destination = "platform=macOS,arch=arm64"
	} else {
		destination = fmt.Sprintf("platform=iOS Simulator,id=%s", udid)
	}

	args := []string{
		"test-without-building",
		"-project", project,
		"-scheme", scheme,
		"-destination", destination,
		"-parallel-testing-enabled", "NO",
	}

	if testTarget != "" {
		args = append(args, "-only-testing:"+testTarget)
	}

	cmd := exec.Command("xcodebuild", args...)

	var combined bytes.Buffer
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &combined
		cmd.Stderr = &combined
	}

	if err := cmd.Run(); err != nil {
		output := combined.String()
		if output != "" {
			// Extract the most useful lines — filter out noise
			var relevant []string
			for _, line := range strings.Split(output, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				// Skip xcodebuild timestamp noise
				if strings.Contains(trimmed, "[MT] IDELaunchParametersSnapshot") {
					continue
				}
				if strings.Contains(trimmed, "[MT] IDETestOperationsObserverDebug") {
					continue
				}
				relevant = append(relevant, trimmed)
			}
			// Show last 15 relevant lines
			if len(relevant) > 15 {
				relevant = relevant[len(relevant)-15:]
			}
			if len(relevant) > 0 {
				return fmt.Errorf("xcodebuild test failed: %w\n%s", err, strings.Join(relevant, "\n"))
			}
		}
		return fmt.Errorf("xcodebuild test failed: %w", err)
	}

	return nil
}

// DeviceCategory returns a short category name for organizing screenshots.
func DeviceCategory(deviceName, platform string) string {
	lower := strings.ToLower(deviceName)
	switch {
	case platform == "macos" || strings.Contains(lower, "macos") || strings.Contains(lower, "mac"):
		return "mac"
	case strings.Contains(lower, "ipad"):
		return "iPad"
	default:
		return "iPhone"
	}
}

// CollectScreenshots moves screenshots from the helper output directory to the
// final destination organized by appearance and device category.
func CollectScreenshots(helperOutputDir, finalDir, appearance, deviceCategory string, verbose bool) (int, error) {
	appearanceDir := filepath.Join(finalDir, appearance, deviceCategory)
	if err := os.MkdirAll(appearanceDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	count := 0
	entries, err := os.ReadDir(helperOutputDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read helper output: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			continue
		}

		src := filepath.Join(helperOutputDir, entry.Name())
		dst := filepath.Join(appearanceDir, entry.Name())

		data, err := os.ReadFile(src)
		if err != nil {
			return count, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return count, fmt.Errorf("failed to write %s: %w", entry.Name(), err)
		}

		if verbose {
			fmt.Printf("    %s\n", entry.Name())
		}
		count++
	}

	return count, nil
}
