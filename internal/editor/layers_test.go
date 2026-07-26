package editor

import (
	"image"
	"image/color"
	"testing"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/render"
)

// The layered preview is only honest if the browser puts a device exactly where
// the renderer would have drawn it. Both use the same arithmetic; this checks
// that they still agree, including the truncation, for offsets that are
// negative, fractional and off-canvas.
func TestLayerPositionMatchesTheRenderer(t *testing.T) {
	const canvasW, canvasH = 400, 800
	img := image.NewNRGBA(image.Rect(0, 0, 120, 240))
	for i := range img.Pix {
		img.Pix[i] = 255 // opaque white
	}

	for _, x := range []float64{0, 37, -37, 12.4, 12.6, -12.4, -12.6, 500, -500} {
		for _, y := range []float64{0, 55, -55, 9.9} {
			d := &config.DeviceImage{X: x, Y: y}

			// What the renderer does.
			dc := gg.NewContext(canvasW, canvasH)
			dc.SetColor(color.NRGBA{0, 0, 0, 255})
			dc.Clear()
			render.DrawDeviceImage(dc, d, img)
			drawn := whiteBounds(dc.Image())

			// What the page is told to do.
			lx, ly := devicePosition(d, canvasW, img.Bounds().Dx())
			// Clipped to the card, as the browser's overflow:hidden does.
			want := image.Rect(lx, ly, lx+120, ly+240).
				Intersect(image.Rect(0, 0, canvasW, canvasH))

			if drawn != want {
				t.Errorf("x=%v y=%v: renderer drew %v, layer says %v", x, y, drawn, want)
			}
		}
	}
}

func whiteBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	out := image.Rectangle{}
	first := true
	for yy := b.Min.Y; yy < b.Max.Y; yy++ {
		for xx := b.Min.X; xx < b.Max.X; xx++ {
			r, g, bl, _ := img.At(xx, yy).RGBA()
			if r>>8 > 200 && g>>8 > 200 && bl>>8 > 200 {
				p := image.Rect(xx, yy, xx+1, yy+1)
				if first {
					out, first = p, false
				} else {
					out = out.Union(p)
				}
			}
		}
	}
	return out
}

// Layer keys are content addresses, so the URL a browser caches must change
// when and only when the pixels do.
func TestLayerKeysTrackContent(t *testing.T) {
	dir := "/screens"
	a := &config.DeviceImage{Source: "a.png", Height: 2400, AutoCrop: true}
	same := &config.DeviceImage{Source: "a.png", Height: 2400, AutoCrop: true}

	if imageKey(a, dir, "iphone") != imageKey(same, dir, "iphone") {
		t.Error("identical configs must share a key, or nothing is ever cached")
	}

	// Moving a device does not change its pixels, and must not change its key —
	// this is what makes a drag cost no network.
	moved := *a
	moved.X, moved.Y = 500, -300
	if imageKey(&moved, dir, "iphone") != imageKey(a, dir, "iphone") {
		t.Error("x/y must not affect the key; a drag would refetch every frame")
	}

	// Anything that does change the pixels must change it.
	for name, mutate := range map[string]func(*config.DeviceImage){
		"height":    func(d *config.DeviceImage) { d.Height = 2401 },
		"width":     func(d *config.DeviceImage) { d.Width = 100 },
		"source":    func(d *config.DeviceImage) { d.Source = "b.png" },
		"template":  func(d *config.DeviceImage) { d.Template = "t.json" },
		"threshold": func(d *config.DeviceImage) { d.CropThreshold = 8 },
		"padding":   func(d *config.DeviceImage) { d.CropPadding = 4 },
		"autocrop":  func(d *config.DeviceImage) { d.AutoCrop = false },
	} {
		changed := *a
		mutate(&changed)
		if imageKey(&changed, dir, "iphone") == imageKey(a, dir, "iphone") {
			t.Errorf("%s changes the pixels but not the key — a stale layer would be served", name)
		}
	}
}

func TestTextLayerKeyCoversEveryFieldRenderTextReads(t *testing.T) {
	base := &config.TextConfig{Text: "Hi", Font: "f.otf", Size: 40, Color: "#fff",
		Align: "center", Y: 10, X: 2, MaxWidth: 100}

	for name, mutate := range map[string]func(*config.TextConfig){
		"text":      func(c *config.TextConfig) { c.Text = "Ho" },
		"font":      func(c *config.TextConfig) { c.Font = "g.otf" },
		"size":      func(c *config.TextConfig) { c.Size = 41 },
		"color":     func(c *config.TextConfig) { c.Color = "#000" },
		"align":     func(c *config.TextConfig) { c.Align = "left" },
		"y":         func(c *config.TextConfig) { c.Y = 11 },
		"x":         func(c *config.TextConfig) { c.X = 3 },
		"max_width": func(c *config.TextConfig) { c.MaxWidth = 101 },
		"backdrop":  func(c *config.TextConfig) { c.Backdrop = &config.TextBackdrop{Color: "#000"} },
	} {
		changed := *base
		mutate(&changed)
		if textKey(&changed) == textKey(base) {
			t.Errorf("%s changes what is drawn but not the key", name)
		}
	}
}
