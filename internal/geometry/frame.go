// Package geometry answers the question the render pipeline never surfaces:
// given a device config, where on the canvas does the *device* actually end up?
//
// It is not obvious, because what gets positioned is not the device. With
// auto_crop, delivr crops the composited frame to its alpha content — and that
// content is the device plus its drop shadow, which Rotato renders offset to one
// side. Centring that box therefore does not centre the device. The shipped
// iphone_front frame is 304px of shadow to the right and 28px to the left: at
// x: 0 the phone sits ~170px left of centre.
//
// Everything here is measurement and arithmetic, deliberately separated from
// the drawing code so it can be tested against known-good numbers.
package geometry

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"

	"github.com/philipparndt/delivr/internal/rotato"
)

// BodySource records how the device body was located, because the two answers
// are not always the same thing and the difference matters to anyone centring
// on it. A phone's screen sits symmetrically inside its bezel, so the mask is a
// fine proxy for the body; something with a chin, or a window with a title bar,
// is a different story.
type BodySource string

const (
	// BodyFromMask means the body box is the bounding box of the frame's screen
	// mask — the authoritative region recorded in the frame JSON's mask_path.
	BodyFromMask BodySource = "mask"
	// BodyFromAlpha means the body box is the bounding box of near-opaque
	// pixels, used when there is no frame mask to consult. Soft shadow is
	// mostly transparent, so a high threshold separates body from shadow.
	BodyFromAlpha BodySource = "alpha"
)

// opaqueThreshold is the alpha above which a pixel is taken to be device rather
// than shadow. Rotato's drop shadows peak well below this; the device body is
// solid.
const opaqueThreshold = 200

// Frame is the measured geometry of one device image source, in that source's
// own native pixels, before any of the config's scaling is applied.
//
// Content is what auto_crop will trim to — device plus shadow. Body is the
// device itself. The gap between them is the whole reason this package exists.
type Frame struct {
	// Size is the native size of the image the pipeline starts from: the frame
	// PNG in template mode, the screenshot itself in flat mode.
	Size image.Point

	// Content is the alpha content box at the device's crop threshold and
	// padding — exactly the box CropToContent will cut. It is the full image
	// when nothing would be trimmed.
	Content image.Rectangle

	// Body is the device silhouette's bounding box.
	Body image.Rectangle

	// BodySource says which measurement Body came from.
	BodySource BodySource

	// Quad is the screen area's four corners (TL, TR, BR, BL) when a frame
	// template describes them. Nil otherwise. On a rotated frame this is what
	// shows the tilt that a bounding box flattens away.
	Quad *[4][2]int

	// Cal, when set, is what the renderer actually did at one particular
	// target size. See Calibration.
	Cal *Calibration
}

// Calibration replaces Place's prediction of the crop with the crop the
// renderer really took.
//
// Place has to guess where an alpha threshold falls in a resampled image, and
// with auto_crop that guess is then multiplied by the second scale — so at
// heavy downscales a one-pixel misjudgement can show up as two or three on the
// canvas. A caller that has already run the pipeline knows the answer and can
// hand it back, which is what makes the editor's overlays exact rather than
// approximate.
//
// It is only valid for the size it was captured at. Place ignores it when the
// device's width or height has moved on, and falls back to predicting — which
// is the right behaviour for a solver searching over heights.
type Calibration struct {
	ForWidth, ForHeight int             // the config width/height this describes
	Stage               image.Point     // image size after the first scale
	Crop                image.Rectangle // the crop taken, in staged pixels
}

// Calibrated returns a copy of the frame carrying a calibration.
func (f Frame) Calibrated(c *Calibration) Frame {
	f.Cal = c
	return f
}

// ShadowLeft and the three that follow report how far the crop box reaches past
// the device body on each side, in native pixels. Asymmetry here is what makes
// a centred crop box an off-centre device.
func (f Frame) ShadowLeft() int   { return f.Body.Min.X - f.Content.Min.X }
func (f Frame) ShadowRight() int  { return f.Content.Max.X - f.Body.Max.X }
func (f Frame) ShadowTop() int    { return f.Body.Min.Y - f.Content.Min.Y }
func (f Frame) ShadowBottom() int { return f.Content.Max.Y - f.Body.Max.Y }

// MeasureFrameTemplate measures a Rotato frame template. The crop threshold
// must match the device config's, since it decides where the content box falls.
//
// crop_padding is deliberately not applied here: the pipeline crops the
// *scaled* image, so the padding is in scaled pixels. Place applies it in the
// right space.
//
// The composited screenshot never extends the content box: it is clipped to the
// screen mask, which lies inside the device body, which lies inside the frame's
// own alpha. So the frame PNG alone is enough to measure, and the measurement
// can be cached per template rather than redone per screenshot.
func MeasureFrameTemplate(jsonPath string, cropThreshold uint8) (Frame, error) {
	frame, mask, meta, err := rotato.LoadFrame(jsonPath)
	if err != nil {
		return Frame{}, fmt.Errorf("failed to load frame template %s: %w", jsonPath, err)
	}

	f := Frame{
		Size:    frame.Bounds().Size(),
		Content: contentBox(frame, cropThreshold),
	}

	if mask != nil {
		if b := alphaBounds(mask, 8); !b.Empty() {
			f.Body = b
			f.BodySource = BodyFromMask
		}
	}
	if f.Body.Empty() {
		f.Body = alphaBounds(frame, opaqueThreshold)
		f.BodySource = BodyFromAlpha
	}
	if meta != nil {
		q := meta.Corners
		f.Quad = &q
	}

	return f, nil
}

// MeasureImage measures a flat image used as a device with no frame template.
// There is no mask to consult, so the body is whatever is near-opaque — for an
// ordinary screenshot with no transparency at all, that is the whole image.
func MeasureImage(path string, cropThreshold uint8) (Frame, error) {
	img, err := imaging.Open(path)
	if err != nil {
		return Frame{}, fmt.Errorf("failed to open image %s: %w", path, err)
	}
	nrgba := imaging.Clone(img)

	f := Frame{
		Size:       nrgba.Bounds().Size(),
		Content:    contentBox(nrgba, cropThreshold),
		Body:       alphaBounds(nrgba, opaqueThreshold),
		BodySource: BodyFromAlpha,
	}
	if f.Body.Empty() {
		f.Body = f.Content
	}
	return f, nil
}

// contentBox is the alpha content bounds CropToContent detects, before
// padding. Threshold 0 means the pipeline's own default of 10.
func contentBox(img image.Image, threshold uint8) image.Rectangle {
	if threshold == 0 {
		threshold = 10
	}
	return rotato.DetectContentBounds(img, threshold)
}

// alphaBounds is the bounding box of pixels above an alpha threshold.
func alphaBounds(img image.Image, threshold uint8) image.Rectangle {
	b := rotato.DetectContentBounds(img, threshold)
	// DetectContentBounds returns the full bounds when it finds nothing, which
	// is the right answer for a crop and the wrong one for a silhouette.
	if b == img.Bounds() && !hasAlphaAbove(img, threshold) {
		return image.Rectangle{}
	}
	return b
}

func hasAlphaAbove(img image.Image, threshold uint8) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); uint8(a>>8) > threshold {
				return true
			}
		}
	}
	return false
}

// ResolveSourcePath mirrors how RenderDevice builds a device image path, so a
// caller measuring a frame is measuring the file that will actually be drawn.
func ResolveSourcePath(screenshotsDir, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(screenshotsDir, rel)
}

// Exists is a small convenience for callers deciding whether a template is
// measurable before trying.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
