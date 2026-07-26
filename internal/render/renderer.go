package render

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/fonts"
)

// Renderer orchestrates screenshot generation
type Renderer struct {
	cfg        *config.Config
	fontLoader *fonts.Loader
	outputDir  string
	verbose    bool
}

// NewRenderer creates a new renderer
func NewRenderer(cfg *config.Config, outputDir string, verbose bool) *Renderer {
	return &Renderer{
		cfg:        cfg,
		fontLoader: fonts.NewLoader(cfg.Settings.FontsDir),
		outputDir:  outputDir,
		verbose:    verbose,
	}
}

// Close releases resources
func (r *Renderer) Close() {
	r.fontLoader.Close()
}

// renderJob describes a single image to render.
type renderJob struct {
	device *config.Device
	screen *config.Screen
	output *config.Output
	lang   string
}

// RenderAll renders all configured outputs in parallel.
func (r *Renderer) RenderAll() error {
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Collect all jobs
	var jobs []renderJob
	languages := r.cfg.Languages
	if len(languages) == 0 {
		languages = []string{""}
	}

	for _, lang := range languages {
		for _, output := range r.cfg.Outputs {
			device, ok := r.cfg.Devices[output.Device]
			if !ok {
				return fmt.Errorf("unknown device: %s", output.Device)
			}
			for _, screenID := range output.Screens {
				screen := r.cfg.GetScreen(screenID)
				if screen == nil {
					return fmt.Errorf("unknown screen: %s", screenID)
				}
				outputCopy := output
				jobs = append(jobs, renderJob{
					device: &device,
					screen: screen,
					output: &outputCopy,
					lang:   lang,
				})
			}
		}
	}

	total := len(jobs)
	workers := runtime.NumCPU()
	fmt.Printf("Rendering %d images (%d workers)\n", total, workers)

	// Set up progress bar
	bar := progress.New(
		progress.WithSolidFill("#1A6B5A"),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	var done atomic.Int64
	var firstErr error
	var errOnce sync.Once
	var mu sync.Mutex // protects progress bar output

	printProgress := func(n int64) {
		pct := float64(n) / float64(total)
		label := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			fmt.Sprintf(" %d/%d", n, total),
		)
		mu.Lock()
		fmt.Printf("\r  %s%s", bar.ViewAs(pct), label)
		if n == int64(total) {
			fmt.Println()
		}
		mu.Unlock()
	}

	// Run jobs in parallel
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := range jobs {
		wg.Add(1)
		go func(j renderJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := r.renderScreen(j.device, j.screen, j.output, j.lang); err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("failed to render %s/%s: %w", j.output.Device, j.screen.ID, err)
				})
				return
			}

			n := done.Add(1)
			printProgress(n)
		}(jobs[i])
	}

	wg.Wait()

	if firstErr != nil {
		fmt.Println() // newline after progress bar
		return firstErr
	}

	return nil
}

// DeviceImageLoader produces the image to draw for a device config.
//
// It exists so a caller can put a cache in front of PrepareDeviceImage without
// growing a second copy of the painting order. `generate` passes the plain
// loader; the editor passes a caching one, so dragging a device does not
// recomposite a 3840x2160 frame on every mouse move.
type DeviceImageLoader func(*config.DeviceImage) (image.Image, error)

// BackgroundLoader returns a pre-rendered background for a screen, or nil to
// have it drawn the usual way.
//
// Like DeviceImageLoader, this exists so a caller can cache. Filling a
// 1320x2868 gradient runs a shader over 3.8 million pixels and costs about
// 160ms, which is most of the budget for an interactive redraw — and it
// produces the same pixels every time the background config has not changed.
type BackgroundLoader func(*config.Background) (image.Image, error)

// PaintScreen paints a screen onto a context: background, then devices back to
// front, then title, then subtitle. This is the whole definition of what a
// delivr screenshot is, and everything that renders one goes through here so
// the editor's preview cannot drift from what `generate` writes.
func PaintScreen(dc *gg.Context, screen *config.Screen, fontLoader *fonts.Loader, load DeviceImageLoader) error {
	return PaintScreenWith(dc, screen, fontLoader, load, nil)
}

// PaintScreenWith is PaintScreen with an optional background cache.
func PaintScreenWith(dc *gg.Context, screen *config.Screen, fontLoader *fonts.Loader,
	load DeviceImageLoader, loadBackground BackgroundLoader) error {

	painted := false
	if loadBackground != nil {
		bg, err := loadBackground(screen.Background)
		if err != nil {
			return fmt.Errorf("background: %w", err)
		}
		painted = bg != nil && blitOpaque(dc, bg)
	}
	if !painted {
		if err := RenderBackground(dc, screen.Background); err != nil {
			return fmt.Errorf("background: %w", err)
		}
	}

	if len(screen.Devices) > 0 {
		// Multiple devices in order (first = back, last = front)
		for i := range screen.Devices {
			img, err := load(&screen.Devices[i])
			if err != nil {
				return fmt.Errorf("device[%d]: %w", i, err)
			}
			DrawDeviceImage(dc, &screen.Devices[i], img)
		}
	} else if screen.Device != nil {
		// Single device mode (backwards compatible)
		img, err := load(screen.Device)
		if err != nil {
			return fmt.Errorf("device: %w", err)
		}
		DrawDeviceImage(dc, screen.Device, img)
	}

	if screen.Title != nil {
		if err := RenderText(dc, screen.Title, fontLoader); err != nil {
			return fmt.Errorf("title: %w", err)
		}
	}

	if screen.Subtitle != nil {
		if err := RenderText(dc, screen.Subtitle, fontLoader); err != nil {
			return fmt.Errorf("subtitle: %w", err)
		}
	}

	return nil
}

// blitOpaque replaces a context's pixels with a pre-rendered image, and reports
// whether it could.
//
// A straight copy rather than DrawImage: the background is the first thing
// painted and covers the whole canvas, so there is nothing underneath to
// composite against, and copying is a memmove where compositing is another pass
// over every pixel. The result is byte-identical to having drawn it here.
func blitOpaque(dc *gg.Context, src image.Image) bool {
	dst, ok := dc.Image().(*image.RGBA)
	if !ok || dst.Bounds() != src.Bounds() {
		return false
	}
	srcRGBA, ok := src.(*image.RGBA)
	if !ok || srcRGBA.Stride != dst.Stride {
		return false
	}
	copy(dst.Pix, srcRGBA.Pix)
	return true
}

// ScreenForLang returns the screen as it renders in a given language, with
// translations and font overrides applied. Exported so callers that paint a
// screen themselves localise it the same way RenderAll does.
func (r *Renderer) ScreenForLang(screen *config.Screen, lang string) *config.Screen {
	return r.applyTranslation(screen, lang)
}

// FontLoader exposes the renderer's font cache, so a caller measuring text
// resolves the same faces the renderer draws with.
func (r *Renderer) FontLoader() *fonts.Loader { return r.fontLoader }

// Config returns the configuration the renderer was built from.
func (r *Renderer) Config() *config.Config { return r.cfg }

// applyTranslation returns a screen copy with translated title/subtitle text and font overrides
func (r *Renderer) applyTranslation(screen *config.Screen, lang string) *config.Screen {
	if lang == "" {
		return screen
	}

	// Look up translation text (may be nil)
	var trans *config.ScreenTranslation
	if r.cfg.Translations != nil {
		if langTranslations, ok := r.cfg.Translations[lang]; ok {
			if t, ok := langTranslations[screen.ID]; ok {
				trans = &t
			}
		}
	}

	// Look up font overrides (may be nil)
	var fontCfg *config.LanguageFontConfig
	if r.cfg.LanguageFonts != nil {
		if fc, ok := r.cfg.LanguageFonts[lang]; ok {
			fontCfg = &fc
		}
	}

	// Nothing to apply
	if trans == nil && fontCfg == nil {
		return screen
	}

	// Create a shallow copy of the screen
	copy := *screen

	if copy.Title != nil {
		titleCopy := *copy.Title
		if trans != nil && trans.Title != "" {
			titleCopy.Text = trans.Title
		}
		if fontCfg != nil {
			if fontCfg.TitleFont != "" {
				titleCopy.Font = fontCfg.TitleFont
			}
			if fontCfg.TitleSize != 0 {
				titleCopy.Size = fontCfg.TitleSize
			}
		}
		copy.Title = &titleCopy
	}

	if copy.Subtitle != nil {
		subtitleCopy := *copy.Subtitle
		if trans != nil && trans.Subtitle != "" {
			subtitleCopy.Text = trans.Subtitle
		}
		if fontCfg != nil {
			if fontCfg.SubtitleFont != "" {
				subtitleCopy.Font = fontCfg.SubtitleFont
			}
			if fontCfg.SubtitleSize != 0 {
				subtitleCopy.Size = fontCfg.SubtitleSize
			}
		}
		copy.Subtitle = &subtitleCopy
	}

	return &copy
}

// renderScreen renders a single screen for a device
func (r *Renderer) renderScreen(device *config.Device, screen *config.Screen, output *config.Output, lang string) error {
	// Apply translation if language is set
	screen = r.applyTranslation(screen, lang)

	// Use screenshot_prefix if set, otherwise use device key
	deviceName := device.ScreenshotPrefix
	if deviceName == "" {
		deviceName = output.Device
	}

	dc := gg.NewContext(device.Width, device.Height)
	load := func(d *config.DeviceImage) (image.Image, error) {
		return PrepareDeviceImage(d, r.cfg.Settings.ScreenshotsDir, deviceName)
	}
	if err := PaintScreen(dc, screen, r.fontLoader, load); err != nil {
		return err
	}

	// Save output (grouped by language/device)
	var deviceDir string
	if lang != "" {
		deviceDir = filepath.Join(r.outputDir, lang, output.Device)
	} else {
		deviceDir = filepath.Join(r.outputDir, output.Device)
	}
	if err := os.MkdirAll(deviceDir, 0755); err != nil {
		return fmt.Errorf("create device dir: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.png", output.Prefix, screen.ID)
	outputPath := filepath.Join(deviceDir, filename)

	if err := dc.SavePNG(outputPath); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	return nil
}
