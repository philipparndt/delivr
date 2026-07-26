package geometry

import (
	"image"
	"math"

	"github.com/philipparndt/delivr/internal/config"
)

// Rect is an axis-aligned box in canvas pixels. Sub-pixel, because the whole
// point is to report where things land rather than where they were asked to go.
type Rect struct {
	X0, Y0, X1, Y1 float64
}

func (r Rect) W() float64  { return r.X1 - r.X0 }
func (r Rect) H() float64  { return r.Y1 - r.Y0 }
func (r Rect) CX() float64 { return (r.X0 + r.X1) / 2 }
func (r Rect) CY() float64 { return (r.Y0 + r.Y1) / 2 }

// Margins is the distance from each device body edge to the matching canvas
// edge. Negative means the body runs past that edge, which for the bottom is
// usually deliberate — a device that stops short of the bottom reads as dropped
// onto the card rather than as part of it.
type Margins struct {
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

// Placement is where one device config puts things on one canvas.
type Placement struct {
	// Image is the rectangle the pipeline actually draws — the cropped,
	// scaled image. Its centre is what `x: 0` centres.
	Image Rect `json:"image"`
	// Content is the auto_crop content box. Identical to Image when auto_crop
	// is on; when it is off, it is where auto_crop *would* cut.
	Content Rect `json:"content"`
	// Body is the device itself: the thing a human means by "the phone".
	Body Rect `json:"body"`
	// Quad is the screen area's four corners, when the frame describes them.
	// On a rotated frame this shows the tilt that Body flattens away.
	Quad *[4][2]float64 `json:"quad,omitempty"`

	ScaleX float64 `json:"scaleX"`
	ScaleY float64 `json:"scaleY"`

	Margins    Margins    `json:"margins"`
	BodySource BodySource `json:"bodySource"`
	Cropped    bool       `json:"cropped"`

	// OffCentre is how far the body's centre sits from the canvas centre.
	// This is the number that was silently 170 for weeks.
	OffCentre float64 `json:"offCentre"`
}

// Place computes where a device config lands on a canvas, reproducing the
// pipeline in internal/render/device.go step for step.
//
// The step worth knowing about is that with auto_crop the source is scaled
// twice: RenderWithFrame scales the whole frame to the target size, then
// CropToContent trims it, then scaleImageToFit scales the *trimmed* image to
// the target size again. So `height` ends up sizing the crop box — device plus
// shadow — and the device inside it comes out proportionally smaller. It also
// means crop_padding is measured in the scaled image's pixels rather than the
// source's, which matters as soon as the two differ.
func Place(f Frame, d *config.DeviceImage, canvas image.Point) Placement {
	if f.Size.X == 0 || f.Size.Y == 0 {
		return Placement{}
	}

	// Stage A: the whole source scaled towards the target size.
	aw, ah := fitDims(f.Size.X, f.Size.Y, d.Width, d.Height)
	stage := image.Rect(0, 0, aw, ah)

	// The content box as it falls in stage-A pixels, standing in for
	// re-detecting alpha bounds on the resampled image. Rounded outwards, not
	// to nearest: a resampling filter spreads an edge into its neighbours, so
	// the threshold crossing in the scaled image lands at or outside where the
	// native box maps to, never inside it. On the shipped iphone_front frame
	// this predicts the box CropToContent actually finds, exactly, on both axes.
	//
	// A calibration captured at this same target size replaces the prediction
	// with what the renderer really did, which is what the editor supplies.
	ax := float64(aw) / float64(f.Size.X)
	ay := float64(ah) / float64(f.Size.Y)
	contentA := expandRect(f.Content, ax, ay).
		Inset(-d.CropPadding).
		Intersect(stage)

	if c := f.Cal; c != nil && c.ForWidth == d.Width && c.ForHeight == d.Height &&
		c.Stage.X > 0 && c.Stage.Y > 0 {
		aw, ah = c.Stage.X, c.Stage.Y
		ax = float64(aw) / float64(f.Size.X)
		ay = float64(ah) / float64(f.Size.Y)
		stage = image.Rect(0, 0, aw, ah)
		contentA = c.Crop
	}

	// The body is a coordinate mapping rather than a detection — the mask is
	// never re-thresholded — so it stays sub-pixel.
	bodyA := scaleFRect(f.Body, ax, ay)

	var (
		fw, fh           int     // the drawn image's size
		bx, by           float64 // stage B scale
		originX, originY float64 // the drawn image's origin, in stage-A pixels
	)
	if d.AutoCrop {
		cw, ch := contentA.Dx(), contentA.Dy()
		if cw <= 0 || ch <= 0 {
			contentA = stage
			cw, ch = aw, ah
		}
		if d.Width > 0 || d.Height > 0 {
			fw, fh = fitDims(cw, ch, d.Width, d.Height)
		} else {
			fw, fh = cw, ch
		}
		bx = float64(fw) / float64(cw)
		by = float64(fh) / float64(ch)
		originX, originY = float64(contentA.Min.X), float64(contentA.Min.Y)
	} else {
		contentA = expandRect(f.Content, ax, ay).Inset(-d.CropPadding).Intersect(stage)
		fw, fh = aw, ah
		bx, by = 1, 1
		originX, originY = 0, 0
	}

	// Centred on the canvas, plus the config's offset. Note that what gets
	// centred is the drawn image — shadow included.
	x0 := (float64(canvas.X)-float64(fw))/2 + d.X
	y0 := d.Y

	toCanvas := func(r Rect) Rect {
		return Rect{
			X0: x0 + (r.X0-originX)*bx,
			Y0: y0 + (r.Y0-originY)*by,
			X1: x0 + (r.X1-originX)*bx,
			Y1: y0 + (r.Y1-originY)*by,
		}
	}

	p := Placement{
		Image:      Rect{X0: x0, Y0: y0, X1: x0 + float64(fw), Y1: y0 + float64(fh)},
		Content:    toCanvas(toFRect(contentA)),
		Body:       toCanvas(bodyA),
		ScaleX:     ax * bx,
		ScaleY:     ay * by,
		BodySource: f.BodySource,
		Cropped:    d.AutoCrop,
	}
	p.Margins = Margins{
		Left:   p.Body.X0,
		Right:  float64(canvas.X) - p.Body.X1,
		Top:    p.Body.Y0,
		Bottom: float64(canvas.Y) - p.Body.Y1,
	}
	p.OffCentre = p.Body.CX() - float64(canvas.X)/2

	if f.Quad != nil {
		var q [4][2]float64
		for i, c := range *f.Quad {
			q[i] = [2]float64{
				x0 + (float64(c[0])*ax-originX)*bx,
				y0 + (float64(c[1])*ay-originY)*by,
			}
		}
		p.Quad = &q
	}

	return p
}

// fitDims mirrors scaleImageToFit / rotato.scaleImage: a zero dimension is
// derived from the other, both zero leaves the size alone, and both set force
// the size regardless of aspect. The int truncation is theirs, kept so the
// model lands on the same pixel they do.
func fitDims(origW, origH, targetW, targetH int) (int, int) {
	if targetW == 0 && targetH == 0 {
		return origW, origH
	}
	if origW == 0 || origH == 0 {
		return origW, origH
	}
	if targetW == 0 {
		scale := float64(targetH) / float64(origH)
		return int(float64(origW) * scale), targetH
	}
	if targetH == 0 {
		scale := float64(targetW) / float64(origW)
		return targetW, int(float64(origH) * scale)
	}
	return targetW, targetH
}

func scaleFRect(r image.Rectangle, sx, sy float64) Rect {
	return Rect{
		X0: float64(r.Min.X) * sx,
		Y0: float64(r.Min.Y) * sy,
		X1: float64(r.Max.X) * sx,
		Y1: float64(r.Max.Y) * sy,
	}
}

func toFRect(r image.Rectangle) Rect {
	return Rect{
		X0: float64(r.Min.X), Y0: float64(r.Min.Y),
		X1: float64(r.Max.X), Y1: float64(r.Max.Y),
	}
}

// expandRect scales a rectangle outwards to whole pixels.
func expandRect(r image.Rectangle, sx, sy float64) image.Rectangle {
	return image.Rect(
		int(math.Floor(float64(r.Min.X)*sx)),
		int(math.Floor(float64(r.Min.Y)*sy)),
		int(math.Ceil(float64(r.Max.X)*sx)),
		int(math.Ceil(float64(r.Max.Y)*sy)),
	)
}
