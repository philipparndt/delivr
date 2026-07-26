package geometry

import (
	"image"
	"math"
	"testing"

	"github.com/philipparndt/delivr/internal/config"
)

// iphoneFront is the measured geometry of the iphone_front Rotato frame that
// the neon-snare marketing repo ships: a 3840x2160 render whose drop shadow
// reaches 304px to the right of the phone and 28px to the left. Every number
// here came from reading the frame PNG's alpha and the frame mask, and the
// asymmetry is the reason x: 0 does not centre anything.
func iphoneFront() Frame {
	return Frame{
		Size:       image.Pt(3840, 2160),
		Content:    image.Rect(1501, 208, 2615, 2160),
		Body:       image.Rect(1529, 230, 2311, 1930),
		BodySource: BodyFromMask,
	}
}

// iphoneCanvas is the 6.9-inch App Store slot.
func iphoneCanvas() image.Point { return image.Pt(1320, 2868) }

func front(x, y float64, height int) *config.DeviceImage {
	return &config.DeviceImage{
		AutoCrop:      true,
		CropThreshold: 8,
		Height:        height,
		X:             x,
		Y:             y,
	}
}

func near(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.2f, want %.2f (±%.2f)", label, got, want, tol)
	}
}

func TestShadowIsAsymmetric(t *testing.T) {
	f := iphoneFront()
	if f.ShadowLeft() != 28 || f.ShadowRight() != 304 {
		t.Fatalf("shadow reach = %d left / %d right, want 28 / 304",
			f.ShadowLeft(), f.ShadowRight())
	}
}

// The headline quirk: x: 0 centres the crop box, and the crop box is the phone
// plus a shadow that only exists on one side.
func TestXZeroDoesNotCentreTheDevice(t *testing.T) {
	p := Place(iphoneFront(), front(0, 438, 2822), iphoneCanvas())

	near(t, "image centre", p.Image.CX(), 660, 0.6)
	if math.Abs(p.OffCentre) < 150 {
		t.Errorf("body off-centre by only %.1fpx; the whole problem is that "+
			"this is large and invisible", p.OffCentre)
	}
	// Left of centre, because the shadow on the right pads the crop box out.
	if p.OffCentre > 0 {
		t.Errorf("body sits right of centre (%.1f); the shadow is on the right, "+
			"so the phone should be pushed left", p.OffCentre)
	}
	// And the visible consequence: a wall of dead background on one side.
	if p.Margins.Right-p.Margins.Left < 300 {
		t.Errorf("margins %.0f left / %.0f right; expected a gap of 300px+",
			p.Margins.Left, p.Margins.Right)
	}
}

// The second quirk: height sizes the cropped image, so y+height is nowhere near
// the bottom of the device.
func TestHeightSizesTheCropBoxNotTheDevice(t *testing.T) {
	d := front(200, 438, 2822)
	p := Place(iphoneFront(), d, iphoneCanvas())

	naive := d.Y + float64(d.Height)
	if math.Abs(p.Body.Y1-naive) < 100 {
		t.Errorf("body bottom %.0f is suspiciously close to y+height %.0f; "+
			"the point is that they differ", p.Body.Y1, naive)
	}
	// The body is shorter than the image it is drawn inside, in the same ratio
	// as the body is to the content box.
	near(t, "body height", p.Body.H(),
		float64(d.Height)*1700.0/1952.0, 3)
}

// The committed neon-snare iphone-front geometry, checked against what the
// config's comments claim it achieves: body top at 470, bottom 60px past the
// canvas, body centred.
func TestShippedIPhoneFrontLayoutIsWhatTheCommentsSay(t *testing.T) {
	p := Place(iphoneFront(), front(200, 438, 2822), iphoneCanvas())

	near(t, "body top", p.Body.Y0, 470, 2)
	near(t, "bleed past bottom", p.Body.Y1-2868, 60, 2)
	near(t, "off centre", p.OffCentre, 0, 2)
}

func TestPlaceWithoutAutoCropUsesTheWholeImage(t *testing.T) {
	f := iphoneFront()
	d := &config.DeviceImage{Height: 2160, X: 0, Y: 0}
	p := Place(f, d, iphoneCanvas())

	// Height 2160 is the frame's own height, so nothing is scaled.
	near(t, "scale", p.ScaleX, 1, 0.01)
	near(t, "image width", p.Image.W(), 3840, 1)
	// The image is centred, and with no crop the body sits wherever it sits in
	// the frame — which for a Rotato render is the middle.
	near(t, "off centre", p.OffCentre, 0, 2)
	// The crop box is still reported, so the editor can show what auto_crop
	// would have done.
	near(t, "content width", p.Content.W(), 1114, 2)
}

func TestPlaceScalesToWidthWhenHeightIsUnset(t *testing.T) {
	d := &config.DeviceImage{AutoCrop: true, CropThreshold: 8, Width: 1114}
	p := Place(iphoneFront(), d, iphoneCanvas())
	near(t, "image width", p.Image.W(), 1114, 1)
	near(t, "scale", p.ScaleX, 1, 0.01)
}

func TestPlaceHandlesAnUnmeasurableFrame(t *testing.T) {
	if got := Place(Frame{}, front(0, 0, 100), iphoneCanvas()); got.Image.W() != 0 {
		t.Errorf("expected a zero placement for a zero frame, got %+v", got)
	}
}
