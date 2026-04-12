package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Job represents a single capture job (one device + one appearance).
type Job struct {
	Device     DeviceConfig
	Appearance string
	UDID       string // empty for macOS
	Runtime    string // e.g. "iOS 26.4"
}

// RunCapture orchestrates the full screenshot capture pipeline.
func RunCapture(cfg *Config, verbose bool) error {
	// Build jobs: device × appearance
	var jobs []Job
	for _, device := range cfg.Devices {
		for _, appearance := range cfg.Appearances {
			job := Job{
				Device:     device,
				Appearance: appearance,
			}

			// Look up simulator UDID for iOS devices
			if device.Platform != "macos" {
				result, err := FindDevice(device.Name)
				if err != nil {
					return fmt.Errorf("device %q: %w", device.Name, err)
				}
				job.UDID = result.UDID
				job.Runtime = result.Runtime
			}

			jobs = append(jobs, job)
		}
	}

	total := len(jobs)
	if total == 0 {
		return fmt.Errorf("no capture jobs: none of the devices in the config have a 'platform' field set (needs 'ios' or 'macos')")
	}

	headerStyle := lipgloss.NewStyle().Bold(true)
	fmt.Println(headerStyle.Render(fmt.Sprintf(
		"Capturing screenshots: %d jobs (%d devices × %d appearances)",
		total, len(cfg.Devices), len(cfg.Appearances),
	)))
	fmt.Println()

	// Build once upfront so parallel test runs don't fight over the build DB
	buildStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	fmt.Println(buildStyle.Render("Building for testing..."))
	if err := BuildForTesting(cfg.Project, cfg.Scheme, cfg.Devices, verbose); err != nil {
		return err
	}
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	fmt.Println(doneStyle.Render("Build complete."))
	fmt.Println()

	tracker := NewProgressTracker(jobs)

	if cfg.Parallel {
		return runParallel(cfg, jobs, tracker, verbose)
	}
	return runSequential(cfg, jobs, tracker, verbose)
}

func runParallel(cfg *Config, jobs []Job, tracker *ProgressTracker, verbose bool) error {
	// Group jobs by device — each device runs appearances sequentially,
	// but different devices run in parallel. A simulator can only be
	// booted once, so we can't run dark+light on the same device simultaneously.
	type deviceGroup struct {
		indices []int
		jobs    []Job
	}
	groups := make(map[string]*deviceGroup)
	var groupOrder []string

	for i, j := range jobs {
		key := j.Device.Name + "|" + j.Device.Platform
		if groups[key] == nil {
			groups[key] = &deviceGroup{}
			groupOrder = append(groupOrder, key)
		}
		groups[key].indices = append(groups[key].indices, i)
		groups[key].jobs = append(groups[key].jobs, j)
	}

	// Initial render
	tracker.Update(0, StepPending)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for _, key := range groupOrder {
		g := groups[key]
		wg.Add(1)
		go func(grp *deviceGroup) {
			defer wg.Done()

			// Run appearances sequentially for this device
			for i, j := range grp.jobs {
				idx := grp.indices[i]
				if err := runJobWithProgress(cfg, j, idx, tracker, verbose); err != nil {
					tracker.Fail(idx, err)
					errOnce.Do(func() {
						firstErr = fmt.Errorf("%s/%s: %w", j.Device.Name, j.Appearance, err)
					})
					return // stop this device on first error
				}
			}
		}(g)
	}

	wg.Wait()
	tracker.Finalize()
	fmt.Println()

	if firstErr != nil {
		// Print the full error details below the progress table
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		fmt.Println(errStyle.Render("Error: " + firstErr.Error()))
		return fmt.Errorf("capture failed")
	}

	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	fmt.Println(doneStyle.Render(fmt.Sprintf("Screenshots saved to %s", cfg.Output)))
	return nil
}

func runSequential(cfg *Config, jobs []Job, tracker *ProgressTracker, verbose bool) error {
	// Initial render
	tracker.Update(0, StepPending)

	for i, job := range jobs {
		if err := runJobWithProgress(cfg, job, i, tracker, verbose); err != nil {
			tracker.Fail(i, err)
			tracker.Finalize()
			fmt.Println()
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
			fmt.Println(errStyle.Render("Error: " + err.Error()))
			return fmt.Errorf("capture failed")
		}
	}

	tracker.Finalize()
	fmt.Println()

	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	fmt.Println(doneStyle.Render(fmt.Sprintf("Screenshots saved to %s", cfg.Output)))
	return nil
}

func runJobWithProgress(cfg *Config, job Job, index int, tracker *ProgressTracker, verbose bool) error {
	var helperDir string
	var err error

	if job.Device.Platform == "macos" {
		// macOS apps run sandboxed — the SnapshotHelper writes screenshots
		// into its sandbox container's caches dir. We pass a placeholder
		// helperDir for config writing; after tests, we scan ~/Library/Containers/
		// to find the actual screenshots.
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("failed to get home dir: %w", homeErr)
		}
		helperDir = filepath.Join(home, "Library", "Caches", "tools.delivr", "output-macos-placeholder")
		os.MkdirAll(helperDir, 0755)
		return runMacOSJobWithProgress(cfg, job, index, helperDir, tracker, verbose)
	}

	// iOS: use a regular temp dir (simulators can access the host filesystem)
	helperDir, err = os.MkdirTemp("", "delivr-capture-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(helperDir)
	return runIOSJobWithProgress(cfg, job, index, helperDir, tracker, verbose)
}

func runIOSJobWithProgress(cfg *Config, job Job, index int, helperDir string, tracker *ProgressTracker, verbose bool) error {
	// Boot simulator
	tracker.Update(index, StepBooting)
	if err := BootDevice(job.UDID); err != nil {
		return fmt.Errorf("failed to boot: %w", err)
	}
	if err := WaitForBoot(job.UDID); err != nil {
		return fmt.Errorf("failed to wait for boot: %w", err)
	}

	// Set status bar
	tracker.Update(index, StepStatusBar)
	if err := SetStatusBar(job.UDID, cfg.StatusBar); err != nil {
		return fmt.Errorf("failed to set status bar: %w", err)
	}

	// Set appearance
	tracker.Update(index, StepAppearance)
	if err := SetAppearance(job.UDID, job.Appearance); err != nil {
		return fmt.Errorf("failed to set appearance: %w", err)
	}

	// Write SnapshotHelper config
	tracker.Update(index, StepWritingConfig)
	if err := WriteHelperConfig(job.UDID, job.Device.Name, helperDir); err != nil {
		return fmt.Errorf("failed to write helper config: %w", err)
	}

	// Run tests with screenshot count polling
	tracker.Update(index, StepRunningTests+" (0 screenshots)")
	stopPolling := pollScreenshots(helperDir, index, tracker)
	testErr := RunTests(cfg.Project, cfg.Scheme, cfg.TestTarget, job.Device.Name, "ios", job.UDID, verbose)
	stopPolling()

	if testErr != nil {
		return testErr
	}

	// Collect screenshots
	count, err := CollectScreenshots(helperDir, cfg.Output, job.Appearance, DeviceCategory(job.Device.Name, job.Device.Platform), verbose)
	if err != nil {
		return fmt.Errorf("failed to collect screenshots: %w", err)
	}

	tracker.Update(index, fmt.Sprintf("Done (%d screenshots)", count))
	return nil
}

func runMacOSJobWithProgress(cfg *Config, job Job, index int, helperDir string, tracker *ProgressTracker, verbose bool) error {
	// Save current macOS appearance to restore later
	originalDarkMode, err := getMacOSDarkMode()
	if err != nil {
		originalDarkMode = false // default to light if we can't read it
	}

	// Write SnapshotHelper config for macOS
	tracker.Update(index, StepWritingConfig)
	if err := WriteHelperConfigMacOS(job.Device.Name, helperDir, cfg.MacOSWindowSize); err != nil {
		return fmt.Errorf("failed to write helper config: %w", err)
	}

	// Set macOS appearance
	tracker.Update(index, StepMacOSAppearance)
	wantDark := job.Appearance == "dark"
	if err := setMacOSDarkMode(wantDark); err != nil {
		return fmt.Errorf("failed to set macOS appearance: %w", err)
	}

	// Ensure we restore the original appearance when done
	defer func() {
		_ = setMacOSDarkMode(originalDarkMode)
	}()

	// Run tests with screenshot count polling.
	// Poll the sandbox container dynamically (it may not exist yet at start).
	tracker.Update(index, StepRunningTests+" (0 screenshots)")
	home, _ := os.UserHomeDir()
	// Clear stale screenshots from any previous run
	if stale := findMacOSContainerScreenshotsDir(home); stale != "" {
		os.RemoveAll(stale)
	}
	stopPolling := pollMacOSScreenshots(home, index, tracker)
	testErr := RunTests(cfg.Project, cfg.Scheme, cfg.TestTarget, job.Device.Name, "macos", "", verbose)
	stopPolling()

	if testErr != nil {
		return testErr
	}

	// On macOS, find screenshots in the sandbox container.
	actualDir := helperDir
	if found := findMacOSContainerScreenshotsDir(home); found != "" && countPNGs(found) > 0 {
		actualDir = found
	}
	defer os.RemoveAll(actualDir)

	// Collect screenshots
	count, err := CollectScreenshots(actualDir, cfg.Output, job.Appearance, DeviceCategory(job.Device.Name, job.Device.Platform), verbose)
	if err != nil {
		return fmt.Errorf("failed to collect screenshots: %w", err)
	}

	tracker.Update(index, fmt.Sprintf("Done (%d screenshots)", count))
	return nil
}

// findMacOSContainerScreenshotsDir finds the sandbox container's screenshots dir.
func findMacOSContainerScreenshotsDir(home string) string {
	containersDir := filepath.Join(home, "Library", "Containers")
	entries, err := os.ReadDir(containersDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		candidate := filepath.Join(containersDir, e.Name(), "Data", "Library", "Caches", "tools.delivr", "screenshots")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func runOsascript(script string) error {
	return exec.Command("osascript", "-e", script).Run()
}

// getMacOSDarkMode returns true if macOS is currently in dark mode.
func getMacOSDarkMode() (bool, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to tell appearance preferences to get dark mode`).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// setMacOSDarkMode sets macOS dark mode on or off.
func setMacOSDarkMode(dark bool) error {
	val := "false"
	if dark {
		val = "true"
	}
	return runOsascript(fmt.Sprintf(
		`tell application "System Events" to tell appearance preferences to set dark mode to %s`, val))
}

// resizeMacOSWindow resizes the frontmost window of the given app to width×height
// and centers it on screen.
func resizeMacOSWindow(appName string, width, height int) error {
	script := fmt.Sprintf(`
tell application "System Events"
	tell process "%s"
		set frontmost to true
		delay 0.5
		tell window 1
			set size to {%d, %d}
			set position to {100, 100}
		end tell
	end tell
end tell`, appName, width, height)
	return runOsascript(script)
}

// pollMacOSScreenshots polls for macOS screenshots by dynamically discovering
// the sandbox container directory on each tick (since it may not exist at start).
func pollMacOSScreenshots(home string, index int, tracker *ProgressTracker) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		lastCount := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				count := 0
				if dir := findMacOSContainerScreenshotsDir(home); dir != "" {
					count = countPNGs(dir)
				}
				if count != lastCount {
					lastCount = count
					tracker.Update(index, fmt.Sprintf("%s (%d screenshots)", StepRunningTests, count))
				}
			}
		}
	}()
	return func() { close(done) }
}

// pollScreenshots polls a directory for .png files and updates the tracker
// with the current count. Returns a stop function.
func pollScreenshots(dir string, index int, tracker *ProgressTracker) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		lastCount := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				count := countPNGs(dir)
				if count != lastCount {
					lastCount = count
					tracker.Update(index, fmt.Sprintf("%s (%d screenshots)", StepRunningTests, count))
				}
			}
		}
	}()
	return func() { close(done) }
}

func countPNGs(dir string) int {
	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Also check subdirectories
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".png") {
				count++
			}
			return nil
		})
		return count
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			count++
		}
	}
	return count
}
