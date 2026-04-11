package capture

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Step constants for job progress reporting.
const (
	StepPending          = "Waiting"
	StepBuilding         = "Building for testing"
	StepBooting          = "Booting simulator"
	StepStatusBar        = "Setting status bar"
	StepAppearance       = "Setting appearance"
	StepWritingConfig    = "Preparing test config"
	StepRunningTests     = "Running UI tests"
	StepCollecting       = "Collecting screenshots"
	StepDone             = "Done"
	StepFailed           = "Failed"
	StepMacOSAppearance  = "Setting macOS appearance"
)

// JobStatus tracks the current state of a capture job.
type JobStatus struct {
	DeviceName string
	Appearance string
	Runtime    string // e.g. "iOS 26.4" or "macOS"
	Step       string
	Error      string
}

// ProgressTracker manages and renders job progress for all capture jobs.
type ProgressTracker struct {
	jobs    []JobStatus
	mu      sync.Mutex
	lines   int // number of lines last rendered (for clearing)

	// Styles
	headerStyle  lipgloss.Style
	deviceStyle  lipgloss.Style
	darkStyle    lipgloss.Style
	lightStyle   lipgloss.Style
	stepStyle    lipgloss.Style
	doneStyle    lipgloss.Style
	failStyle    lipgloss.Style
	pendingStyle lipgloss.Style
	bracketStyle lipgloss.Style
}

// NewProgressTracker creates a tracker for the given jobs.
func NewProgressTracker(jobs []Job) *ProgressTracker {
	statuses := make([]JobStatus, len(jobs))
	for i, j := range jobs {
		runtime := j.Runtime
		if j.Device.Platform == "macos" {
			runtime = "macOS"
		}
		statuses[i] = JobStatus{
			DeviceName: j.Device.Name,
			Appearance: j.Appearance,
			Runtime:    runtime,
			Step:       StepPending,
		}
	}

	return &ProgressTracker{
		jobs:         statuses,
		headerStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		deviceStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		darkStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		lightStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		stepStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		doneStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		failStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		pendingStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		bracketStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

// Update sets the step for a specific job and re-renders.
func (p *ProgressTracker) Update(index int, step string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.jobs[index].Step = step
	p.render()
}

// Fail marks a job as failed and re-renders.
func (p *ProgressTracker) Fail(index int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.jobs[index].Step = StepFailed
	p.jobs[index].Error = err.Error()
	p.render()
}

// render clears the previous output and redraws all job statuses.
func (p *ProgressTracker) render() {
	// Move cursor up to clear previous render
	if p.lines > 0 {
		fmt.Printf("\033[%dA", p.lines)
	}

	var sb strings.Builder

	// Calculate column widths for alignment
	maxDeviceLen := 0
	maxAppearanceLen := 0
	maxRuntimeLen := 0
	for _, j := range p.jobs {
		if len(j.DeviceName) > maxDeviceLen {
			maxDeviceLen = len(j.DeviceName)
		}
		if len(j.Appearance) > maxAppearanceLen {
			maxAppearanceLen = len(j.Appearance)
		}
		if len(j.Runtime) > maxRuntimeLen {
			maxRuntimeLen = len(j.Runtime)
		}
	}

	for _, j := range p.jobs {
		// Pad plain text to fixed widths, then style
		devicePadded := j.DeviceName + strings.Repeat(" ", maxDeviceLen-len(j.DeviceName))
		appearancePadded := j.Appearance + strings.Repeat(" ", maxAppearanceLen-len(j.Appearance))
		runtimePadded := j.Runtime + strings.Repeat(" ", maxRuntimeLen-len(j.Runtime))

		styledDevice := p.deviceStyle.Render(devicePadded)

		var styledAppearance string
		if j.Appearance == "dark" {
			styledAppearance = p.darkStyle.Render(appearancePadded)
		} else {
			styledAppearance = p.lightStyle.Render(appearancePadded)
		}

		styledRuntime := p.pendingStyle.Render(runtimePadded)

		// Style the step
		var styledStep string
		switch j.Step {
		case StepDone:
			styledStep = p.doneStyle.Render("Done")
		case StepFailed:
			// Show only a short error in the status line
			errMsg := j.Error
			if idx := strings.Index(errMsg, "\n"); idx > 0 {
				errMsg = errMsg[:idx]
			}
			if len(errMsg) > 60 {
				errMsg = errMsg[:60] + "..."
			}
			styledStep = p.failStyle.Render("Failed: " + errMsg)
		case StepPending:
			styledStep = p.pendingStyle.Render("Waiting")
		default:
			styledStep = p.stepStyle.Render(j.Step)
		}

		// Build the status icon
		var icon string
		switch j.Step {
		case StepDone:
			icon = p.doneStyle.Render("✓")
		case StepFailed:
			icon = p.failStyle.Render("✗")
		case StepPending:
			icon = p.pendingStyle.Render("○")
		default:
			icon = p.stepStyle.Render("●")
		}

		line := fmt.Sprintf("  %s  %s  %s  %s  %s%s%s",
			icon,
			styledDevice,
			styledAppearance,
			styledRuntime,
			p.bracketStyle.Render("["),
			styledStep,
			p.bracketStyle.Render("]"),
		)
		sb.WriteString(line)
		sb.WriteString("\033[K\n") // clear to end of line
	}

	output := sb.String()
	fmt.Print(output)
	p.lines = len(p.jobs)
}

// Finalize prints the final state without ANSI cursor manipulation.
func (p *ProgressTracker) Finalize() {
	// The last render is already in place, just ensure cursor is at the end
}
