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
func (p *Processor) Process(raw string, dev config.VideoDeviceConfig, audio *config.VideoAudioConfig, outPath string) error {
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
	}
	// Audio. There is always a track: App Store Connect rejects a preview with
	// no audio stream at all, and reports it as "unsupported or corrupted
	// audio" — which reads as a codec fault rather than an absent stream. With
	// no soundtrack configured that track is silence; with one, it is the music.
	// Do not "optimise" either back to -an.
	var afilter string
	if audio != nil && audio.Track != "" {
		args = append(args, "-ss", fmt.Sprintf("%.3f", audio.Offset), "-i", audio.Track)
		afilter = audioFilter(audio, duration)
	} else {
		args = append(args, "-f", "lavfi",
			"-i", "anullsrc=channel_layout=stereo:sample_rate=44100")
	}
	args = append(args,
		"-t", fmt.Sprintf("%.3f", duration),
		"-map", "0:v:0", "-map", "1:a:0",
	)
	if afilter != "" {
		args = append(args, "-af", afilter)
	}
	args = append(args, []string{
		"-vf", vf,
		"-c:v", "libx264",
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-r", "30",
		// Apple re-encodes anyway; a high bitrate here just avoids compounding
		// artefacts through two passes.
		"-b:v", "12M", "-maxrate", "16M", "-bufsize", "24M",
		"-c:a", "aac", "-b:a", "128k", "-ar", "44100",
		"-movflags", "+faststart",
		outPath,
	}...)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("ffmpeg produced no output: %w", err)
	}
	// Checked rather than assumed: a missing audio track is the one defect
	// App Store Connect will not tell you about until after an upload.
	if err := requireAudioTrack(outPath); err != nil {
		return err
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

// requireAudioTrack fails when the encoded preview has no audio stream.
func requireAudioTrack(path string) error {
	out, err := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "a", "-show_entries", "stream=codec_type",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		// ffprobe ships with ffmpeg; if it is somehow absent, do not fail the
		// encode over a check.
		return nil
	}
	if !strings.Contains(string(out), "audio") {
		return fmt.Errorf("%s has no audio track — App Store Connect rejects "+
			"previews without one", filepath.Base(path))
	}
	return nil
}

// audioFilter builds the -af chain: pad short tracks to length, then fade.
func audioFilter(a *config.VideoAudioConfig, duration float64) string {
	// apad first — a track shorter than the preview would otherwise end early
	// and leave the tail with no audio stream, which is the rejection this all
	// exists to avoid.
	parts := []string{"apad"}
	if a.Volume > 0 && a.Volume != 1 {
		parts = append(parts, fmt.Sprintf("volume=%.3f", a.Volume))
	}
	if a.FadeIn > 0 {
		parts = append(parts, fmt.Sprintf("afade=t=in:st=0:d=%.3f", a.FadeIn))
	}
	if a.FadeOut > 0 {
		start := duration - a.FadeOut
		if start < 0 {
			start = 0
		}
		parts = append(parts, fmt.Sprintf("afade=t=out:st=%.3f:d=%.3f", start, a.FadeOut))
	}
	return strings.Join(parts, ",")
}
