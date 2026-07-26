package geometry

import (
	"image"
	"math"

	"github.com/philipparndt/delivr/internal/config"
)

// CenterX returns the x that puts the device body's centre on the canvas's
// centre, leaving height and y alone.
//
// This is not 0, and on a frame with an offset drop shadow it is not close to
// 0. Place() is linear in x, so one placement is enough to solve it exactly.
func CenterX(f Frame, d *config.DeviceImage, canvas image.Point) float64 {
	p := Place(f, d, canvas)
	if p.Body.W() == 0 {
		return d.X
	}
	return d.X + (float64(canvas.X)/2 - p.Body.CX())
}

// BleedBottom returns the y that runs the device body's bottom edge `bleed`
// pixels past the bottom of the canvas, leaving x and height alone. A bleed of
// 0 lands the body exactly on the edge.
//
// Solving this rather than guessing matters because `height` sizes the cropped
// image — device plus shadow — so `y + height` is not the bottom of the device
// and can be a couple of hundred pixels out.
func BleedBottom(f Frame, d *config.DeviceImage, canvas image.Point, bleed float64) float64 {
	p := Place(f, d, canvas)
	if p.Body.H() == 0 {
		return d.Y
	}
	return d.Y + (float64(canvas.Y) + bleed - p.Body.Y1)
}

// Fit solves jointly for the height and y that put the body's top edge at
// `top` and its bottom edge `bleed` pixels past the bottom of the canvas.
//
// Height is the awkward one: it is a knob on the cropped image, so its effect
// on the body is scaled by the ratio between the content box and the body. The
// analytic estimate below is exact up to the integer truncation inside the two
// resize steps, and the short search around it picks the integer that actually
// lands closest.
func Fit(f Frame, d *config.DeviceImage, canvas image.Point, top, bleed float64) (height int, y float64) {
	want := float64(canvas.Y) + bleed - top
	if want <= 0 || f.Body.Dy() == 0 {
		return d.Height, d.Y
	}

	probe := *d
	estimate := estimateHeight(f, &probe, want)

	best := estimate
	bestErr := math.Inf(1)
	for h := estimate - 3; h <= estimate+3; h++ {
		if h <= 0 {
			continue
		}
		probe.Height = h
		if e := math.Abs(Place(f, &probe, canvas).Body.H() - want); e < bestErr {
			best, bestErr = h, e
		}
	}

	probe.Height = best
	p := Place(f, &probe, canvas)
	return best, d.Y + (top - p.Body.Y0)
}

// estimateHeight inverts the body-height relationship analytically. With
// auto_crop the total scale is height/contentHeight, so a body of
// bodyHeight × scale tall needs height = want × contentHeight / bodyHeight.
// Without auto_crop, height sizes the whole source instead.
func estimateHeight(f Frame, d *config.DeviceImage, want float64) int {
	ref := f.Size.Y
	if d.AutoCrop && f.Content.Dy() > 0 {
		ref = f.Content.Dy()
	}
	if f.Body.Dy() == 0 {
		return d.Height
	}
	return max(1, int(math.Round(want*float64(ref)/float64(f.Body.Dy()))))
}

// DefaultSeamGap is the gap the App Store leaves between screenshots in its
// carousel, as a fraction of one screenshot's width. It is an approximation of
// what the store does, but it is much closer than the zero people assume.
const DefaultSeamGap = 0.04

// SeamGapFor returns the default gap in pixels for a canvas width.
func SeamGapFor(canvasWidth int) float64 {
	return math.Round(float64(canvasWidth) * DefaultSeamGap)
}

// SeamMatchX returns the x that continues a device from the neighbouring
// screenshot into this one.
//
// `right` says which side this screenshot is on: true when this screenshot is
// to the right of the neighbour, which is the usual case — a device that runs
// off screenshot 1's right edge comes back in from screenshot 2's left.
//
// The gap is the term everyone forgets. Without it the device appears to jump
// backwards by the width of the carousel gap as the reader scrolls across the
// boundary, which is small enough to look like a rendering artefact and large
// enough to be obvious.
func SeamMatchX(neighbourX float64, canvasWidth int, gap float64, right bool) float64 {
	step := float64(canvasWidth) + gap
	if right {
		return neighbourX - step
	}
	return neighbourX + step
}

// SeamError reports how far a device is from continuing cleanly across the
// seam: 0 means the two halves line up, positive means this screenshot's copy
// sits that many pixels to the right of where the neighbour's would carry it.
func SeamError(neighbourX, thisX float64, canvasWidth int, gap float64, right bool) float64 {
	return thisX - SeamMatchX(neighbourX, canvasWidth, gap, right)
}
