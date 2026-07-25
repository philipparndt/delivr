// Package video records, cuts and encodes App Store preview videos.
//
// Apple's constraints are tight and only checked after upload: 15 to 30
// seconds, a fixed resolution per device family, H.264 or ProRes. Getting one
// wrong means a rejection hours later, so everything here aims to produce a
// file that is already correct rather than one that probably is.
package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/philipparndt/delivr/internal/capture"
	"github.com/philipparndt/delivr/internal/config"
)

// Recorder drives a simulator and films it.
type Recorder struct {
	Verbose bool
	Log     func(string, ...any)
}

func (r *Recorder) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

// Record films one device's preview and returns the path to the raw capture.
//
// The raw file is deliberately longer than the finished preview: trimming needs
// material either side, and simulator launch takes a variable amount of time
// that a fixed offset cannot absorb.
func (r *Recorder) Record(dev config.VideoDeviceConfig, rec config.VideoRecordConfig,
	simulatorName, bundleID, outDir string) (string, error) {

	name := simulatorName
	if dev.Simulator != "" {
		name = dev.Simulator
	}
	found, err := capture.FindDevice(name)
	if err != nil {
		return "", fmt.Errorf("simulator %q: %w", name, err)
	}

	r.logf("booting %s", name)
	if err := capture.BootDevice(found.UDID); err != nil {
		return "", err
	}
	if err := capture.WaitForBoot(found.UDID); err != nil {
		return "", err
	}

	if rec.App != "" {
		r.logf("installing %s", filepath.Base(rec.App))
		if out, err := exec.Command("xcrun", "simctl", "install", found.UDID, rec.App).CombinedOutput(); err != nil {
			return "", fmt.Errorf("install failed: %w: %s", err, out)
		}
	}

	_ = exec.Command("xcrun", "simctl", "terminate", found.UDID, bundleID).Run()

	// A first run pays for shader compilation and texture caches. Filming that
	// means the hitching is the first thing a viewer sees, so spend it here.
	if rec.WarmUp {
		r.logf("warm-up pass")
		if err := launch(found.UDID, bundleID, rec.LaunchArgs); err != nil {
			return "", err
		}
		time.Sleep(time.Duration(rec.Duration*0.45) * time.Second)
		_ = exec.Command("xcrun", "simctl", "terminate", found.UDID, bundleID).Run()
		time.Sleep(1 * time.Second)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	raw := filepath.Join(outDir, "raw.mov")

	r.logf("recording %.0fs", rec.Duration)
	// Start the recorder first, then launch: the reverse loses the opening.
	cmd := exec.Command("xcrun", "simctl", "io", found.UDID, "recordVideo",
		"--codec", "h264", "--force", raw)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("recorder failed to start: %w", err)
	}
	time.Sleep(1 * time.Second)

	if err := launch(found.UDID, bundleID, rec.LaunchArgs); err != nil {
		_ = cmd.Process.Kill()
		return "", err
	}
	time.Sleep(time.Duration(rec.Duration * float64(time.Second)))

	// SIGINT, not Kill: simctl finalises the container on interrupt, and a
	// killed recording leaves an unplayable file.
	_ = cmd.Process.Signal(os.Interrupt)
	_ = cmd.Wait()
	time.Sleep(1 * time.Second)
	_ = exec.Command("xcrun", "simctl", "terminate", found.UDID, bundleID).Run()

	info, err := os.Stat(raw)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("recording produced no usable file at %s", raw)
	}
	r.logf("captured %.1f MB", float64(info.Size())/1024/1024)
	return raw, nil
}

func launch(udid, bundleID string, args []string) error {
	argv := append([]string{"simctl", "launch", udid, bundleID}, args...)
	if out, err := exec.Command("xcrun", argv...).CombinedOutput(); err != nil {
		return fmt.Errorf("launch failed: %w: %s", err, out)
	}
	return nil
}
