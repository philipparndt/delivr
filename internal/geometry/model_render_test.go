package geometry_test

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/geometry"
	"github.com/philipparndt/delivr/internal/render"
)

// The geometry package models the render pipeline rather than running it, which
// is only worth anything if the model and the pipeline agree. These tests build
// a frame with a deliberately lopsided shadow, push it through the real
// renderer, and check that the device landed where the model said it would.
//
// The synthetic frame is used instead of a real Rotato export so the test
// carries its own fixtures and the expected numbers are arithmetic rather than
// measurements of somebody's private assets.

const (
	frameW, frameH = 800, 600
	// The device body, opaque in the frame PNG.
	bodyX0, bodyY0, bodyX1, bodyY1 = 200, 100, 400, 500
	// The screen, which the frame punches transparent and the mask marks.
	screenX0, screenY0, screenX1, screenY1 = 210, 110, 390, 490
	// A soft drop shadow, reaching 160px to the right and nothing to the left.
	// This is the whole point: it makes the content box lopsided around the
	// device, exactly as Rotato's own shadows do.
	shadowX1, shadowY1 = 560, 540
)

// writeSyntheticFrame builds a frame PNG, its screen mask, the metadata JSON
// and a screenshot to composite into it. The screenshot is pure red so the
// device's screen can be located in the output by colour.
func writeSyntheticFrame(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	frame := image.NewNRGBA(image.Rect(0, 0, frameW, frameH))
	mask := image.NewNRGBA(image.Rect(0, 0, frameW, frameH))

	// Shadow first: low alpha, offset right and down.
	for y := bodyY0 + 30; y < shadowY1; y++ {
		for x := bodyX0; x < shadowX1; x++ {
			frame.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 40})
		}
	}
	// Body: opaque bezel.
	for y := bodyY0; y < bodyY1; y++ {
		for x := bodyX0; x < bodyX1; x++ {
			frame.SetNRGBA(x, y, color.NRGBA{90, 90, 90, 255})
		}
	}
	// Screen: transparent in the frame, opaque in the mask.
	for y := screenY0; y < screenY1; y++ {
		for x := screenX0; x < screenX1; x++ {
			frame.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 0})
			mask.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}

	shot := image.NewNRGBA(image.Rect(0, 0, screenX1-screenX0, screenY1-screenY0))
	for y := shot.Bounds().Min.Y; y < shot.Bounds().Max.Y; y++ {
		for x := shot.Bounds().Min.X; x < shot.Bounds().Max.X; x++ {
			shot.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
		}
	}

	writePNG(t, filepath.Join(dir, "d.frame.png"), frame)
	writePNG(t, filepath.Join(dir, "d.frame.mask.png"), mask)
	writePNG(t, filepath.Join(dir, "shot.png"), shot)

	meta := map[string]any{
		"frame_path":     "d.frame.png",
		"mask_path":      "d.frame.mask.png",
		"frame_width":    frameW,
		"frame_height":   frameH,
		"corners":        [4][2]int{{screenX0, screenY0}, {screenX1, screenY0}, {screenX1, screenY1}, {screenX0, screenY1}},
		"is_rectangle":   true,
		"rectangle_rect": [4]int{screenX0, screenY0, screenX1 - screenX0, screenY1 - screenY0},
	}
	buf, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.frame.json"), buf, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// redBounds finds the screen in a rendered canvas by looking for the red the
// screenshot was filled with. Edges are anti-aliased into the bezel, so the
// test only counts pixels that are still convincingly red.
func redBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 > 180 && g>>8 < 70 && bl>>8 < 70 {
				found = true
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if !found {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// paint runs the real render pipeline for one device on one canvas, and reports
// what the pipeline did on the way so the test can also check the calibrated
// path.
func paint(t *testing.T, dir string, d *config.DeviceImage, canvas image.Point) (image.Image, *geometry.Calibration) {
	t.Helper()
	dc := gg.NewContext(canvas.X, canvas.Y)
	dc.SetColor(color.NRGBA{0, 0, 255, 255})
	dc.Clear()

	img, info, err := render.PrepareDeviceImageInfo(d, dir, "device")
	if err != nil {
		t.Fatal(err)
	}
	render.DrawDeviceImage(dc, d, img)
	return dc.Image(), &geometry.Calibration{
		ForWidth: d.Width, ForHeight: d.Height,
		Stage: info.Stage, Crop: info.Crop,
	}
}

func measure(t *testing.T, dir string, d *config.DeviceImage) geometry.Frame {
	t.Helper()
	f, err := geometry.MeasureFrameTemplate(
		filepath.Join(dir, "d.frame.json"), uint8(d.CropThreshold))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func syntheticDevice() *config.DeviceImage {
	return &config.DeviceImage{
		Source:        "shot.png",
		Template:      "d.frame.json",
		AutoCrop:      true,
		CropThreshold: 8,
		Height:        400,
		Y:             100,
	}
}

func TestMeasuredFrameSeesTheLopsidedShadow(t *testing.T) {
	dir := writeSyntheticFrame(t)
	f := measure(t, dir, syntheticDevice())

	if f.BodySource != geometry.BodyFromMask {
		t.Errorf("body source = %q, want the frame mask", f.BodySource)
	}
	if got := (image.Rect(screenX0, screenY0, screenX1, screenY1)); f.Body != got {
		t.Errorf("body = %v, want the mask bounds %v", f.Body, got)
	}
	if f.ShadowLeft() != 10 || f.ShadowRight() != 170 {
		t.Errorf("shadow reach = %d left / %d right, want 10 / 170",
			f.ShadowLeft(), f.ShadowRight())
	}
}

// The model's prediction against the pipeline's actual pixels.
func TestModelPredictsWhereTheRendererPutsTheDevice(t *testing.T) {
	dir := writeSyntheticFrame(t)
	canvas := image.Pt(400, 800)

	for _, tc := range []struct {
		name   string
		mutate func(*config.DeviceImage)
	}{
		{"at x zero", func(d *config.DeviceImage) {}},
		{"pushed right", func(d *config.DeviceImage) { d.X = 120 }},
		{"pushed left and down", func(d *config.DeviceImage) { d.X = -60; d.Y = 250 }},
		{"taller", func(d *config.DeviceImage) { d.Height = 620 }},
		{"shorter", func(d *config.DeviceImage) { d.Height = 240 }},
		{"sized by width", func(d *config.DeviceImage) { d.Height = 0; d.Width = 300 }},
		{"with crop padding", func(d *config.DeviceImage) { d.CropPadding = 12 }},
		{"without auto crop", func(d *config.DeviceImage) { d.AutoCrop = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := syntheticDevice()
			tc.mutate(d)

			f := measure(t, dir, d)
			canvasImg, cal := paint(t, dir, d, canvas)
			got := redBounds(canvasImg)
			if got.Empty() {
				t.Fatal("the device did not render at all")
			}

			// Predicting the crop means guessing where an alpha threshold
			// lands in a resampled image, and auto_crop then multiplies that
			// guess by the second scale. 3px covers it. Handed the crop the
			// renderer actually took, the model has nothing left to guess.
			check(t, "predicted", got, geometry.Place(f, d, canvas).Body, canvas, 3)
			check(t, "calibrated", got, geometry.Place(f.Calibrated(cal), d, canvas).Body, canvas, 1)
		})
	}
}

// check compares a rendered box against a modelled one, ignoring edges that
// fall outside the canvas — the renderer clips there, so there is nothing left
// to measure.
func check(t *testing.T, label string, got image.Rectangle, want geometry.Rect, canvas image.Point, tol float64) {
	t.Helper()
	for _, e := range []struct {
		edge      string
		got, want float64
		extent    int
	}{
		{"left", float64(got.Min.X), want.X0, canvas.X},
		{"top", float64(got.Min.Y), want.Y0, canvas.Y},
		{"right", float64(got.Max.X), want.X1, canvas.X},
		{"bottom", float64(got.Max.Y), want.Y1, canvas.Y},
	} {
		// An edge the model puts off the canvas has been clipped away in the
		// render, so there is no rendered position to compare it against.
		if e.want < tol || e.want > float64(e.extent)-tol {
			continue
		}
		if math.Abs(e.got-e.want) > tol {
			t.Errorf("%s: %s edge rendered at %.0f, model said %.1f", label, e.edge, e.got, e.want)
		}
	}
}

// The quirk, demonstrated end to end: x: 0 does not centre the device, and the
// solver's answer does.
func TestCentreItFixesWhatXZeroDoesNot(t *testing.T) {
	dir := writeSyntheticFrame(t)
	canvas := image.Pt(400, 800)
	d := syntheticDevice()
	f := measure(t, dir, d)

	img, _ := paint(t, dir, d, canvas)
	before := redBounds(img)
	offBefore := float64(before.Min.X+before.Max.X)/2 - float64(canvas.X)/2
	if math.Abs(offBefore) < 40 {
		t.Fatalf("expected x: 0 to be badly off centre, but it was only %.1fpx out", offBefore)
	}

	d.X = geometry.CenterX(f, d, canvas)
	img, _ = paint(t, dir, d, canvas)
	after := redBounds(img)
	offAfter := float64(after.Min.X+after.Max.X)/2 - float64(canvas.X)/2
	if math.Abs(offAfter) > 2 {
		t.Errorf("after solving, still %.1fpx off centre (x = %.1f)", offAfter, d.X)
	}
}

// And the same for the bottom edge, where `height` is the misleading number.
func TestBleedBottomLandsTheRealBottomEdge(t *testing.T) {
	dir := writeSyntheticFrame(t)
	canvas := image.Pt(400, 800)
	d := syntheticDevice()
	f := measure(t, dir, d)

	d.Y = geometry.BleedBottom(f, d, canvas, 0)
	img, _ := paint(t, dir, d, canvas)
	got := redBounds(img)
	if math.Abs(float64(got.Max.Y-canvas.Y)) > 2 {
		t.Errorf("device bottom at %d, want the canvas edge %d", got.Max.Y, canvas.Y)
	}
	// y + height would have been nowhere near it.
	if math.Abs(d.Y+float64(d.Height)-float64(canvas.Y)) < 20 {
		t.Error("this fixture no longer demonstrates that y+height is misleading")
	}
}
