package editor

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/render"
)

// Dragging a device does not change a single pixel of it. It changes where it
// is. Re-rendering the whole canvas to find that out costs ~160ms, which caps
// the drag at six frames a second and feels exactly as bad as it sounds.
//
// So the preview is sent as layers instead of as one flattened image: the
// background, each device, and the text, each a PNG the browser caches by a
// content-addressed URL. Moving a device is then a CSS offset on an image the
// browser already has — no request, no render, no encode.
//
// The pixels are still the real renderer's. Only the compositing moves to the
// browser, and the position it composites at is the same integer arithmetic
// DrawDeviceImage does, so the layered view and the flattened one agree. On
// mouse-up the server re-renders anyway and the numbers come back authoritative.

// Layer is one image to stack, positioned in frame coordinates.
type Layer struct {
	Kind string `json:"kind"` // "background", "device" or "text"
	Key  string `json:"key"`  // content address; the browser caches on this
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	// Index identifies which device this is, or -1.
	Index int `json:"index"`
	// Screen says which of the two screens in a seam view it belongs to, so
	// the page only moves the layers of the one being edited.
	Screen string `json:"screen"`
}

// layerKey content-addresses a layer so its URL can be cached forever.
func layerKey(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// storeLayer encodes an image once and keeps the bytes under its key.
func (c *previewCache) storeLayer(key string, img image.Image) error {
	c.mu.Lock()
	_, exists := c.layers[key]
	c.mu.Unlock()
	if exists {
		return nil
	}

	buf, err := encodePNGBytes(img)
	if err != nil {
		return err
	}

	c.mu.Lock()
	put(c.layers, &c.layerOrder, key, buf, maxLayers)
	c.mu.Unlock()
	return nil
}

// Layer returns the stored bytes for a key.
func (c *previewCache) Layer(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	buf, ok := c.layers[key]
	return buf, ok
}

// buildLayers produces the stack for one screen at an offset within the frame.
func (s *Server) buildLayers(screen *config.Screen, screenID string, device config.Device,
	deviceName string, offset image.Point, lang string) ([]Layer, []*cachedImage, error) {

	cfg := s.project.Config()
	dir := cfg.Settings.ScreenshotsDir
	canvas := image.Pt(device.Width, device.Height)

	var layers []Layer
	var used []*cachedImage

	// Background. Keyed on the config and the size, so all seven screens of a
	// gradient family that happen to match share one image.
	bg, err := s.cache.background(screen.Background, canvas)
	if err != nil {
		return nil, nil, fmt.Errorf("background: %w", err)
	}
	if bg != nil {
		key := layerKey("bg", backgroundKey(screen.Background, canvas))
		if err := s.cache.storeLayer(key, bg); err != nil {
			return nil, nil, err
		}
		layers = append(layers, Layer{
			Kind: "background", Key: key, Index: -1, Screen: screenID,
			X: offset.X, Y: offset.Y, W: canvas.X, H: canvas.Y,
		})
	}

	// Devices, back to front.
	for i, d := range DevicesOf(screen) {
		entry, err := s.cache.deviceImage(d, dir, deviceName)
		if err != nil {
			return nil, nil, fmt.Errorf("device[%d]: %w", i, err)
		}
		used = append(used, entry)

		key := layerKey("dev", imageKey(d, dir, deviceName))
		if err := s.cache.storeLayer(key, entry.img); err != nil {
			return nil, nil, err
		}
		size := entry.img.Bounds().Size()
		x, y := devicePosition(d, canvas.X, size.X)
		layers = append(layers, Layer{
			Kind: "device", Key: key, Index: i, Screen: screenID,
			X: offset.X + x, Y: offset.Y + y, W: size.X, H: size.Y,
		})
	}

	// Text, on top, as one transparent overlay.
	text, key, err := s.textLayer(screen, canvas, lang)
	if err != nil {
		return nil, nil, err
	}
	if text != nil {
		layers = append(layers, Layer{
			Kind: "text", Key: key, Index: -1, Screen: screenID,
			X: offset.X, Y: offset.Y, W: canvas.X, H: canvas.Y,
		})
	}

	return layers, used, nil
}

// devicePosition is DrawDeviceImage's placement arithmetic, including its
// truncation to whole pixels. The page repeats this exactly while dragging, so
// a layered preview lands on the same pixel a flattened one would.
func devicePosition(d *config.DeviceImage, canvasWidth, imageWidth int) (int, int) {
	return int((float64(canvasWidth)-float64(imageWidth))/2 + d.X), int(d.Y)
}

// textLayer draws the title and subtitle onto a transparent canvas.
func (s *Server) textLayer(screen *config.Screen, canvas image.Point, lang string) (image.Image, string, error) {
	if (screen.Title == nil || screen.Title.Text == "") &&
		(screen.Subtitle == nil || screen.Subtitle.Text == "") {
		return nil, "", nil
	}

	key := layerKey("text", lang, fmt.Sprintf("%dx%d", canvas.X, canvas.Y),
		textKey(screen.Title), textKey(screen.Subtitle))

	c := s.cache
	c.mu.Lock()
	_, cached := c.layers[key]
	c.mu.Unlock()
	if cached {
		return placeholderImage, key, nil
	}

	dc := gg.NewContext(canvas.X, canvas.Y)
	if screen.Title != nil {
		if err := render.RenderText(dc, screen.Title, s.renderer.FontLoader()); err != nil {
			return nil, "", fmt.Errorf("title: %w", err)
		}
	}
	if screen.Subtitle != nil {
		if err := render.RenderText(dc, screen.Subtitle, s.renderer.FontLoader()); err != nil {
			return nil, "", fmt.Errorf("subtitle: %w", err)
		}
	}
	img := dc.Image()
	if err := c.storeLayer(key, img); err != nil {
		return nil, "", err
	}
	return img, key, nil
}

// placeholderImage stands in when a layer is already cached and only its
// existence matters to the caller.
var placeholderImage = image.NewNRGBA(image.Rect(0, 0, 1, 1))

func textKey(t *config.TextConfig) string {
	if t == nil {
		return "-"
	}
	// Every field RenderText reads, and nothing else.
	backdrop := "-"
	if t.Backdrop != nil {
		backdrop = fmt.Sprintf("%s|%s|%g|%g|%g", t.Backdrop.Type, t.Backdrop.Color,
			t.Backdrop.Padding, t.Backdrop.Radius, t.Backdrop.Height)
	}
	return fmt.Sprintf("%s|%s|%g|%s|%s|%g|%g|%g|%s",
		t.Text, t.Font, t.Size, t.Color, t.Align, t.Y, t.X, t.MaxWidth, backdrop)
}

func backgroundKey(bg *config.Background, canvas image.Point) string {
	if bg == nil {
		return "-"
	}
	return fmt.Sprintf("%dx%d|%s|%s|%g|%v", canvas.X, canvas.Y,
		bg.Type, bg.Color, bg.Angle, bg.Stops)
}
