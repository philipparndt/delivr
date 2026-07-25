package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/philipparndt/delivr/internal/config"
)

// Processor cuts a raw recording down to an App Store preview.
type Processor struct {
	Verbose bool
	Log     func(string, ...any)
}

func (p *Processor) logf(format string, args ...any) {
	if p.Log != nil {
		p.Log(format, args...)
	}
}

var blackEnd = regexp.MustCompile(`black_end:([0-9.]+)`)

// Process trims, scales and encodes `raw` into a preview at Apple's spec.
func (p *Processor) Process(raw string, dev config.VideoDeviceConfig, outPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg is required to cut previews: %w", err)
	}

	duration := dev.Duration
	if duration <= 0 {
		duration = 28
	}
	if duration < 15 || duration > 30 {
		// Caught here rather than by App Store Connect hours after upload.
		return fmt.Errorf("preview duration %.1fs is outside Apple's 15-30s range", duration)
	}

	start := dev.Start
	if dev.AutoTrim {
		anchor, err := p.detectStart(raw)
		if err != nil {
			p.logf("auto-trim found no opening black, using start=%.2f: %v", start, err)
		} else {
			p.logf("auto-trim anchor at %.2fs", anchor)
			start += anchor
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	// yuv420p and an even scale: App Store Connect rejects other pixel formats,
	// and H.264 cannot encode odd dimensions.
	vf := fmt.Sprintf("scale=%d:%d,setsar=1", even(dev.Width), even(dev.Height))
	args := []string{
		"-y", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", start),
		"-i", raw,
		"-t", fmt.Sprintf("%.3f", duration),
		"-vf", vf,
		"-c:v", "libx264",
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-r", "30",
		// Apple re-encodes anyway; a high bitrate here just avoids compounding
		// artefacts through two passes.
		"-b:v", "12M", "-maxrate", "16M", "-bufsize", "24M",
		"-movflags", "+faststart",
		"-an", // previews may carry audio, but simctl records none
		outPath,
	}
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("ffmpeg produced no output: %w", err)
	}
	p.logf("%s  %dx%d  %.0fs  %.1f MB", filepath.Base(outPath),
		even(dev.Width), even(dev.Height), duration, float64(info.Size())/1024/1024)
	return nil
}

// detectStart finds the end of the opening black, so the cut lands on the same
// frame every run.
//
// Simulator boot and launch take a variable amount of time — a fixed offset
// drifts by a second or more between runs, which is the difference between
// opening on the title and opening on an empty grid.
func (p *Processor) detectStart(raw string) (float64, error) {
	out, _ := exec.Command("ffmpeg",
		"-t", "10", "-i", raw,
		"-vf", "blackdetect=d=0.35:pix_th=0.08",
		"-an", "-f", "null", "-").CombinedOutput()

	matches := blackEnd.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("no black segment detected in the first 10s")
	}
	// The last one inside the probe window: the app draws its own opening cover,
	// and that is the boundary worth cutting on.
	last := matches[len(matches)-1][1]
	return strconv.ParseFloat(last, 64)
}

// even rounds down to an even number; H.264 cannot encode odd dimensions.
func even(n int) int {
	if n%2 != 0 {
		return n - 1
	}
	return n
}
