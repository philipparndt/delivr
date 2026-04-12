package capture

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Pipe both stdout and stderr into a single reader to display a
	// scrolling tail of the last 3 build output lines.
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("failed to start xcodebuild: %w", err)
	}

	// Close the write end in the parent so the reader gets EOF when the child exits.
	pw.Close()

	var allLines []string
	tail := newBuildTail(3)

	// Read output byte-by-byte into lines to avoid any buffering delay
	// from bufio.Scanner when xcodebuild writes partial lines.
	var lineBuf []byte
	buf := make([]byte, 4096)
	for {
		n, readErr := pr.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' {
					trimmed := strings.TrimSpace(string(lineBuf))
					if trimmed != "" {
						allLines = append(allLines, trimmed)
						tail.push(trimmed)
					}
					lineBuf = lineBuf[:0]
				} else {
					lineBuf = append(lineBuf, b)
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	// Flush remaining partial line
	if trimmed := strings.TrimSpace(string(lineBuf)); trimmed != "" {
		allLines = append(allLines, trimmed)
		tail.push(trimmed)
	}
	pr.Close()

	tail.clear()

	if err := cmd.Wait(); err != nil {
		if len(allLines) > 10 {
			allLines = allLines[len(allLines)-10:]
		}
		if len(allLines) > 0 {
			return fmt.Errorf("build-for-testing failed: %w\n%s", err, strings.Join(allLines, "\n"))
		}
		return fmt.Errorf("build-for-testing failed: %w", err)
	}

	return nil
}

// buildTail renders the last N lines of build output in a fixed-height
// region using ANSI cursor movement, creating a scrolling progress window.
type buildTail struct {
	lines    []string
	maxLines int
	rendered int // number of lines currently rendered on screen
	style    lipgloss.Style
	divStyle lipgloss.Style
}

func newBuildTail(n int) *buildTail {
	return &buildTail{
		maxLines: n,
		style:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		divStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

func (t *buildTail) push(line string) {
	t.lines = append(t.lines, line)
	if len(t.lines) > t.maxLines {
		t.lines = t.lines[len(t.lines)-t.maxLines:]
	}
	t.render()
}

func (t *buildTail) render() {
	// Move cursor up to clear previous render
	if t.rendered > 0 {
		fmt.Printf("\033[%dA", t.rendered)
	}

	termWidth := getTerminalWidth()
	divider := t.divStyle.Render(strings.Repeat("─", termWidth))

	var sb strings.Builder

	// Top divider
	sb.WriteString(divider)
	sb.WriteString("\033[K\n")

	// Tail lines (always render maxLines rows for stable height)
	for i := 0; i < t.maxLines; i++ {
		if i < len(t.lines) {
			display := t.lines[i]
			if len(display) > termWidth-2 {
				display = display[:termWidth-3] + "…"
			}
			sb.WriteString("  ")
			sb.WriteString(t.style.Render(display))
		}
		sb.WriteString("\033[K\n")
	}

	// Bottom divider
	sb.WriteString(divider)
	sb.WriteString("\033[K\n")

	fmt.Print(sb.String())
	t.rendered = t.maxLines + 2 // lines + 2 dividers
}

func (t *buildTail) clear() {
	if t.rendered > 0 {
		fmt.Printf("\033[%dA", t.rendered)
		for i := 0; i < t.rendered; i++ {
			fmt.Print("\033[K\n")
		}
		fmt.Printf("\033[%dA", t.rendered)
	}
}

// RunTests executes xcodebuild test-without-building for a specific device.
// If onLine is non-nil, each line of output is passed to it (for progress display).
func RunTests(project, scheme, testTarget, deviceName, platform, udid string, verbose bool, onLine func(string)) error {
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

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else if onLine != nil {
		// Stream output line-by-line for progress tracking
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		combined := io.MultiReader(stdoutPipe, stderrPipe)

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start xcodebuild: %w", err)
		}

		scanner := bufio.NewScanner(combined)
		var outputLines []string
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				outputLines = append(outputLines, trimmed)
				if !isXcodebuildNoise(trimmed) {
					onLine(trimmed)
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			return formatTestError(err, outputLines)
		}
		return nil
	} else {
		var combined bytes.Buffer
		cmd.Stdout = &combined
		cmd.Stderr = &combined

		if err := cmd.Run(); err != nil {
			return formatTestError(err, strings.Split(combined.String(), "\n"))
		}
		return nil
	}

	return nil
}

// isXcodebuildNoise returns true for lines that are not useful for progress display.
func isXcodebuildNoise(line string) bool {
	return strings.Contains(line, "[MT] IDELaunchParametersSnapshot") ||
		strings.Contains(line, "[MT] IDETestOperationsObserverDebug")
}

// formatTestError extracts useful lines from xcodebuild output for the error message.
func formatTestError(err error, lines []string) error {
	var relevant []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isXcodebuildNoise(trimmed) {
			continue
		}
		relevant = append(relevant, trimmed)
	}
	if len(relevant) > 15 {
		relevant = relevant[len(relevant)-15:]
	}
	if len(relevant) > 0 {
		return fmt.Errorf("xcodebuild test failed: %w\n%s", err, strings.Join(relevant, "\n"))
	}
	return fmt.Errorf("xcodebuild test failed: %w", err)
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
