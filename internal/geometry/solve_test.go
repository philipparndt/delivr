package geometry

import (
	"math"
	"testing"

	"github.com/philipparndt/delivr/internal/config"
)

// The number the neon-snare config records as measured by hand at height 2400,
// with a comment warning that it is 170 and not 0. Recovering it from the frame
// geometry is the point of the solver: nobody should have to measure it again.
func TestCenterXRecoversTheHandMeasuredCorrection(t *testing.T) {
	d := front(0, 470, 2400)
	got := CenterX(iphoneFront(), d, iphoneCanvas())
	near(t, "solved x at height 2400", got, 170, 1)
}

// The same solve at the height the config actually ships.
func TestCenterXMatchesTheCommittedValue(t *testing.T) {
	d := front(0, 438, 2822)
	got := CenterX(iphoneFront(), d, iphoneCanvas())
	near(t, "solved x at height 2822", got, 200, 1.5)
}

func TestCenterXActuallyCentres(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	for _, h := range []int{1800, 2400, 2822, 3200} {
		d := front(-500, 438, h)
		d.X = CenterX(f, d, canvas)
		if off := Place(f, d, canvas).OffCentre; math.Abs(off) > 1 {
			t.Errorf("height %d: still %.2fpx off centre after solving", h, off)
		}
	}
}

// Solving from a wildly wrong starting point must land in the same place as
// solving from a nearly-right one: the model is linear in x, so one step is
// always enough.
func TestCenterXIsIndependentOfWhereItStarts(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	a := CenterX(f, front(-5000, 438, 2822), canvas)
	b := CenterX(f, front(9999, 438, 2822), canvas)
	near(t, "solve from far left vs far right", a, b, 0.001)
}

func TestBleedBottomRunsTheBodyPastTheEdge(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	d := front(200, 0, 2822)

	d.Y = BleedBottom(f, d, canvas, 60)
	near(t, "solved y", d.Y, 438, 2)
	near(t, "body bottom", Place(f, d, canvas).Body.Y1, 2868+60, 0.01)
}

func TestBleedBottomWithNoBleedLandsOnTheEdge(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	d := front(200, 0, 2822)
	d.Y = BleedBottom(f, d, canvas, 0)
	near(t, "body bottom", Place(f, d, canvas).Body.Y1, 2868, 0.01)
}

// Fit solves the pair of numbers the config comment describes as "solved, not
// nudged": the height and y that put the body's top at 470 and run its bottom
// 60px past the canvas.
func TestFitSolvesTheShippedHeightAndY(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	h, y := Fit(f, front(200, 438, 2822), canvas, 470, 60)

	if math.Abs(float64(h-2822)) > 2 {
		t.Errorf("solved height = %d, want 2822 (±2)", h)
	}
	near(t, "solved y", y, 438, 2)
}

func TestFitPutsTheBodyExactlyWhereAsked(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	for _, tc := range []struct{ top, bleed float64 }{
		{470, 60}, {300, 0}, {800, 200}, {100, -100},
	} {
		d := front(0, 0, 2000)
		d.Height, d.Y = Fit(f, d, canvas, tc.top, tc.bleed)
		d.X = CenterX(f, d, canvas)
		p := Place(f, d, canvas)

		near(t, "body top", p.Body.Y0, tc.top, 1.5)
		near(t, "body bottom", p.Body.Y1, 2868+tc.bleed, 1.5)
		near(t, "off centre", p.OffCentre, 0, 1)
	}
}

func TestFitRefusesAnImpossibleRequest(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	d := front(0, 100, 2000)
	// A body top below the bottom of the canvas leaves no height to solve for.
	h, y := Fit(f, d, canvas, 3000, -200)
	if h != d.Height || y != d.Y {
		t.Errorf("expected the values to be left alone, got height %d y %.0f", h, y)
	}
}

// The seam. The committed pair of screens runs one phone across the boundary
// between two screenshots at x: 348 and x: -1025, and the difference between
// them is not one card width — it is one card width plus the gap the App Store
// leaves in its carousel.
func TestSeamMatchXIncludesTheCarouselGap(t *testing.T) {
	got := SeamMatchX(348, 1320, 53, true)
	near(t, "continuation x", got, -1025, 0.01)

	// Without the gap term the device jumps backwards by exactly the gap,
	// which is the mistake this solver exists to prevent.
	naive := SeamMatchX(348, 1320, 0, true)
	near(t, "the mistake", naive-got, 53, 0.01)
}

func TestSeamMatchXGoesBothWays(t *testing.T) {
	right := SeamMatchX(348, 1320, 53, true)
	if back := SeamMatchX(right, 1320, 53, false); math.Abs(back-348) > 0.01 {
		t.Errorf("round trip = %.2f, want 348", back)
	}
}

func TestSeamErrorIsZeroForTheCommittedPair(t *testing.T) {
	near(t, "seam error", SeamError(348, -1025, 1320, 53, true), 0, 0.01)
	// And reports the size of the mistake when the gap was forgotten.
	near(t, "forgotten gap", SeamError(348, -972, 1320, 53, true), 53, 0.01)
}

func TestDefaultSeamGapIsFourPercent(t *testing.T) {
	// 53px on the 6.9-inch slot, which is the number the config arrived at.
	near(t, "gap", SeamGapFor(1320), 53, 0.01)
	near(t, "gap", SeamGapFor(1920), 77, 0.01)
}

func TestSolversLeaveAZeroFrameAlone(t *testing.T) {
	d := front(12, 34, 100)
	if got := CenterX(Frame{}, d, iphoneCanvas()); got != 12 {
		t.Errorf("CenterX = %v, want the original 12", got)
	}
	if got := BleedBottom(Frame{}, d, iphoneCanvas(), 0); got != 34 {
		t.Errorf("BleedBottom = %v, want the original 34", got)
	}
}

func TestSolversWorkWithoutAutoCrop(t *testing.T) {
	f, canvas := iphoneFront(), iphoneCanvas()
	d := &config.DeviceImage{Height: 2000, X: -800, Y: 0}

	d.X = CenterX(f, d, canvas)
	d.Y = BleedBottom(f, d, canvas, 0)
	p := Place(f, d, canvas)

	near(t, "off centre", p.OffCentre, 0, 1)
	near(t, "body bottom", p.Body.Y1, 2868, 0.01)
}
