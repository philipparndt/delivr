package editor

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/geometry"
	"github.com/philipparndt/delivr/internal/render"
)

// The preview is the real renderer. Anything else — a reimplementation in the
// browser, a cheap approximation at a quarter size — would eventually disagree
// with what `generate` writes, and a positioning tool that lies about position
// is worse than no tool.
//
// Done naively that costs about 600ms for one iPhone screen, which is unusable
// to drag. Measured, the time went to three places, and each is cached against
// exactly what it depends on and nothing more:
//
//   - ~350ms compositing the screenshot into a 3840x2160 frame. Depends on the
//     source and the frame, not on the size — so a height sweep pays it once.
//   - ~190ms finding the alpha bounds and rescaling. Depends on the size, not
//     on x or y — so a drag does not pay it at all.
//   - ~160ms filling the background gradient. Depends on the background config
//     and the canvas size, so it is paid once per screen and then memcopied.
//
// What is left for a drag is a copy, a blit and two strings.

// previewCache holds device images at two stages, plus measured frames.
//
// Two stages because the two knobs have different costs. Dragging changes
// nothing about the pixels, so the fully sized image is reused outright.
// Changing the height invalidates that, but not the composite underneath it —
// and the composite is where nearly all the time goes.
type previewCache struct {
	mu         sync.Mutex
	images     map[string]*cachedImage
	composites map[string]image.Image
	frames     map[string]geometry.Frame
	// canvases holds whole rendered screens that cannot change while the user
	// works: the neighbour in a seam view, which deliberately renders from the
	// config as committed rather than from the edits in flight.
	canvases map[string]image.Image
	// backgrounds holds filled gradients, which are pure functions of the
	// background config and the canvas size.
	backgrounds map[string]image.Image
	// layers holds encoded PNGs served by content address, so the browser
	// caches them and a drag refetches nothing.
	layers map[string][]byte

	// insertion order per map, for eviction.
	imageOrder, compositeOrder, backgroundOrder, canvasOrder, layerOrder []string
}

type cachedImage struct {
	img  image.Image
	info render.DeviceImageInfo
}

// Cached images are large — a composited 3840x2160 frame is 33MB, a filled
// canvas 15MB — so the caches are bounded. Without a cap, browsing a
// seven-screen project at two device slots and three languages would hold
// several gigabytes by lunchtime.
const (
	maxComposites  = 6
	maxSizedImages = 16
	maxBackgrounds = 6
	maxCanvases    = 3
	// Encoded layers are small next to the images they came from, and the
	// browser only benefits from a cache hit if the server still has the
	// bytes, so this one is generous.
	maxLayers = 64
)

// put stores a value and evicts the oldest entry once the map is over its cap.
// Insertion order rather than true recency: the access pattern here is a user
// moving between adjacent screens, where the two are much the same thing.
func put[T any](m map[string]T, order *[]string, key string, value T, cap int) {
	if _, exists := m[key]; !exists {
		*order = append(*order, key)
	}
	m[key] = value
	for len(*order) > cap {
		oldest := (*order)[0]
		*order = (*order)[1:]
		delete(m, oldest)
	}
}

func newPreviewCache() *previewCache {
	return &previewCache{
		images:      make(map[string]*cachedImage),
		composites:  make(map[string]image.Image),
		frames:      make(map[string]geometry.Frame),
		canvases:    make(map[string]image.Image),
		backgrounds: make(map[string]image.Image),
		layers:      make(map[string][]byte),
	}
}

// Reset drops everything, for when the config has been rewritten underneath us.
func (c *previewCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.images = make(map[string]*cachedImage)
	c.composites = make(map[string]image.Image)
	c.frames = make(map[string]geometry.Frame)
	c.canvases = make(map[string]image.Image)
	c.backgrounds = make(map[string]image.Image)
	c.layers = make(map[string][]byte)
	c.imageOrder, c.compositeOrder = nil, nil
	c.backgroundOrder, c.canvasOrder, c.layerOrder = nil, nil, nil
}

// background renders and caches a screen's background at the canvas size.
func (c *previewCache) background(bg *config.Background, canvas image.Point) (image.Image, error) {
	if bg == nil {
		return nil, nil
	}
	key := backgroundKey(bg, canvas)

	c.mu.Lock()
	hit, ok := c.backgrounds[key]
	c.mu.Unlock()
	if ok {
		return hit, nil
	}

	dc := gg.NewContext(canvas.X, canvas.Y)
	if err := render.RenderBackground(dc, bg); err != nil {
		return nil, err
	}
	img := dc.Image()

	c.mu.Lock()
	put(c.backgrounds, &c.backgroundOrder, key, img, maxBackgrounds)
	c.mu.Unlock()
	return img, nil
}

// imageKey covers every input to the sized image. X and Y are deliberately
// absent: they are what the drag changes, and they do not affect the pixels.
func imageKey(d *config.DeviceImage, dir, deviceName string) string {
	return fmt.Sprintf("%s|%d|%d|%t|%d|%d", compositeKey(d, dir, deviceName),
		d.Width, d.Height, d.AutoCrop, d.CropThreshold, d.CropPadding)
}

// compositeKey covers the size-independent half.
func compositeKey(d *config.DeviceImage, dir, deviceName string) string {
	return fmt.Sprintf("%s|%s|%s|%s", dir, deviceName, d.Source, d.Template)
}

func (c *previewCache) deviceImage(d *config.DeviceImage, dir, deviceName string) (*cachedImage, error) {
	key := imageKey(d, dir, deviceName)

	c.mu.Lock()
	hit, ok := c.images[key]
	c.mu.Unlock()
	if ok {
		return hit, nil
	}

	composite, err := c.composite(d, dir, deviceName)
	if err != nil {
		return nil, err
	}
	img, info := render.SizeDeviceImage(composite, d)
	entry := &cachedImage{img: img, info: info}

	c.mu.Lock()
	put(c.images, &c.imageOrder, key, entry, maxSizedImages)
	c.mu.Unlock()
	return entry, nil
}

func (c *previewCache) composite(d *config.DeviceImage, dir, deviceName string) (image.Image, error) {
	key := compositeKey(d, dir, deviceName)

	c.mu.Lock()
	hit, ok := c.composites[key]
	c.mu.Unlock()
	if ok {
		return hit, nil
	}

	img, err := render.CompositeDeviceImage(d, dir, deviceName)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	put(c.composites, &c.compositeOrder, key, img, maxComposites)
	c.mu.Unlock()
	return img, nil
}

// frame measures the source's native geometry. Independent of size, so one
// measurement serves every height the user tries.
func (c *previewCache) frame(d *config.DeviceImage, dir, deviceName string) (geometry.Frame, error) {
	threshold := uint8(d.CropThreshold)
	path := render.DeviceSourcePath(d, dir, deviceName)
	if d.Template != "" {
		path = dir + "/" + d.Template
	}
	key := fmt.Sprintf("%s|%d", path, threshold)

	c.mu.Lock()
	hit, ok := c.frames[key]
	c.mu.Unlock()
	if ok {
		return hit, nil
	}

	var (
		f   geometry.Frame
		err error
	)
	if d.Template != "" {
		f, err = geometry.MeasureFrameTemplate(path, threshold)
	} else {
		f, err = geometry.MeasureImage(path, threshold)
	}
	if err != nil {
		return geometry.Frame{}, err
	}

	c.mu.Lock()
	c.frames[key] = f
	c.mu.Unlock()
	return f, nil
}

// paint renders one screen at full canvas size through the real pipeline, and
// reports what the pipeline did with each device so the overlays can be exact
// rather than predicted.
func (c *previewCache) paint(r *render.Renderer, screen *config.Screen, device config.Device,
	deviceName string, dir string) (image.Image, []*cachedImage, error) {

	dc := gg.NewContext(device.Width, device.Height)

	var used []*cachedImage
	load := func(d *config.DeviceImage) (image.Image, error) {
		entry, err := c.deviceImage(d, dir, deviceName)
		if err != nil {
			return nil, err
		}
		used = append(used, entry)
		return entry.img, nil
	}

	canvas := image.Pt(device.Width, device.Height)
	loadBackground := func(bg *config.Background) (image.Image, error) {
		return c.background(bg, canvas)
	}

	if err := render.PaintScreenWith(dc, screen, r.FontLoader(), load, loadBackground); err != nil {
		return nil, nil, err
	}
	return dc.Image(), used, nil
}

// paintCached is paint for a screen that cannot change while the user works.
func (c *previewCache) paintCached(key string, r *render.Renderer, screen *config.Screen,
	device config.Device, deviceName, dir string) (image.Image, error) {

	c.mu.Lock()
	hit, ok := c.canvases[key]
	c.mu.Unlock()
	if ok {
		return hit, nil
	}

	img, _, err := c.paint(r, screen, device, deviceName, dir)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	put(c.canvases, &c.canvasOrder, key, img, maxCanvases)
	c.mu.Unlock()
	return img, nil
}

// displayScale is how much a logical canvas has to shrink to fit the window.
func displayScale(w, h, maxW, maxH int) float64 {
	if maxW <= 0 {
		maxW = 720
	}
	if maxH <= 0 {
		maxH = 1600
	}
	if w <= maxW && h <= maxH {
		return 1
	}
	return min(float64(maxW)/float64(w), float64(maxH)/float64(h))
}

// downscale shrinks an image for display. The filter is a display choice only —
// every number the editor reports comes from the full-size render, never from
// this.
func downscale(img image.Image, scale float64) image.Image {
	if scale >= 1 {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return imaging.Resize(img, int(float64(w)*scale), int(float64(h)*scale), imaging.Linear)
}

// encodePNG produces a data URL. BestSpeed because this is a preview being
// regenerated on every mouse-up, not an asset being shipped.
func encodePNG(img image.Image) (string, error) {
	buf, err := encodePNGBytes(img)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf), nil
}

func encodePNGBytes(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// seamGapColor is what fills the gap between two screenshots in the seam view.
// Neutral and obviously not part of either card, so the eye reads across it
// rather than treating it as background.
var seamGapColor = color.NRGBA{28, 30, 34, 255}

// composeSeam lays two rendered screens side by side with a gap between them,
// in the order they appear on the store page.
//
// Called with the halves already shrunk for display: laying out two 3.8-megapixel
// canvases and then throwing nine tenths of the result away costs a few hundred
// milliseconds, and composition is pure layout — no pixel is combined with
// another — so doing it after the downscale looks the same and is not.
func composeSeam(left, right image.Image, gap int) image.Image {
	lb, rb := left.Bounds(), right.Bounds()
	h := max(lb.Dy(), rb.Dy())
	w := lb.Dx() + gap + rb.Dx()

	dc := gg.NewContext(w, h)
	dc.SetColor(seamGapColor)
	dc.Clear()
	dc.DrawImage(left, 0, 0)
	dc.DrawImage(right, lb.Dx()+gap, 0)
	return dc.Image()
}
