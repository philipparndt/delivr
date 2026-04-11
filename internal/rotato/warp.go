package rotato

import (
	"fmt"
	"image"
	"math"
)

// Homography is a 3x3 matrix stored in row-major order that maps
// (x, y) -> (u, v) via projective division:
//
//	u = (h0*x + h1*y + h2) / (h6*x + h7*y + h8)
//	v = (h3*x + h4*y + h5) / (h6*x + h7*y + h8)
type Homography [9]float64

// ComputeHomography returns the projective transform that maps src[i] to
// dst[i] for i = 0..3. It works by solving an 8x8 linear system using
// Gaussian elimination with partial pivoting.
//
// Both src and dst contain 4 points in the same order (conventionally
// TL, TR, BR, BL).
func ComputeHomography(src, dst [4][2]float64) (Homography, error) {
	// We build the usual 8x9 system. For each correspondence
	// (x, y) -> (u, v) we add two rows:
	//
	//	[ x  y  1  0  0  0  -u*x  -u*y ] * h = [ u ]
	//	[ 0  0  0  x  y  1  -v*x  -v*y ]       [ v ]
	//
	// where h is the first 8 elements of the homography (h8 = 1).
	var a [8][8]float64
	var b [8]float64
	for i := 0; i < 4; i++ {
		x, y := src[i][0], src[i][1]
		u, v := dst[i][0], dst[i][1]
		r0 := i * 2
		r1 := i*2 + 1
		a[r0] = [8]float64{x, y, 1, 0, 0, 0, -u * x, -u * y}
		b[r0] = u
		a[r1] = [8]float64{0, 0, 0, x, y, 1, -v * x, -v * y}
		b[r1] = v
	}
	solved, err := solve8x8(a, b)
	if err != nil {
		return Homography{}, err
	}
	return Homography{
		solved[0], solved[1], solved[2],
		solved[3], solved[4], solved[5],
		solved[6], solved[7], 1.0,
	}, nil
}

// Invert returns H^-1. Fails if the matrix is singular.
func (h Homography) Invert() (Homography, error) {
	// Standard 3x3 matrix inversion using the adjugate / determinant.
	a := h[0]
	b := h[1]
	c := h[2]
	d := h[3]
	e := h[4]
	f := h[5]
	g := h[6]
	hh := h[7]
	i := h[8]

	det := a*(e*i-f*hh) - b*(d*i-f*g) + c*(d*hh-e*g)
	if math.Abs(det) < 1e-12 {
		return Homography{}, fmt.Errorf("homography is singular")
	}
	invDet := 1.0 / det

	return Homography{
		(e*i - f*hh) * invDet, (c*hh - b*i) * invDet, (b*f - c*e) * invDet,
		(f*g - d*i) * invDet, (a*i - c*g) * invDet, (c*d - a*f) * invDet,
		(d*hh - e*g) * invDet, (b*g - a*hh) * invDet, (a*e - b*d) * invDet,
	}, nil
}

// Apply maps (x, y) through the homography.
func (h Homography) Apply(x, y float64) (float64, float64) {
	w := h[6]*x + h[7]*y + h[8]
	if w == 0 {
		return 0, 0
	}
	u := (h[0]*x + h[1]*y + h[2]) / w
	v := (h[3]*x + h[4]*y + h[5]) / w
	return u, v
}

// solve8x8 performs Gaussian elimination with partial pivoting on an 8x8
// linear system Ax = b.
func solve8x8(a [8][8]float64, b [8]float64) ([8]float64, error) {
	const n = 8
	// Forward elimination.
	for col := 0; col < n; col++ {
		// Find pivot row.
		pivot := col
		pivotVal := math.Abs(a[col][col])
		for r := col + 1; r < n; r++ {
			if v := math.Abs(a[r][col]); v > pivotVal {
				pivot = r
				pivotVal = v
			}
		}
		if pivotVal < 1e-12 {
			return [8]float64{}, fmt.Errorf("system is singular at column %d", col)
		}
		if pivot != col {
			a[col], a[pivot] = a[pivot], a[col]
			b[col], b[pivot] = b[pivot], b[col]
		}

		// Eliminate below.
		for r := col + 1; r < n; r++ {
			factor := a[r][col] / a[col][col]
			if factor == 0 {
				continue
			}
			for c := col; c < n; c++ {
				a[r][c] -= factor * a[col][c]
			}
			b[r] -= factor * b[col]
		}
	}

	// Back substitution.
	var x [8]float64
	for r := n - 1; r >= 0; r-- {
		sum := b[r]
		for c := r + 1; c < n; c++ {
			sum -= a[r][c] * x[c]
		}
		x[r] = sum / a[r][r]
	}
	return x, nil
}

// WarpPerspective draws `src` into `dst` so that its four corners land on
// the given destination quadrilateral. Bilinear sampling is used so the
// result looks clean even at substantial rotations. Only pixels inside
// the destination quad are touched.
//
// dstQuad is in TL, TR, BR, BL order, same as FrameMetadata.Corners.
func WarpPerspective(src *image.NRGBA, dst *image.NRGBA, dstQuad [4][2]int) error {
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return fmt.Errorf("source image is empty")
	}

	// Source quad is the full source image in TL, TR, BR, BL order.
	srcQuad := [4][2]float64{
		{0, 0},
		{float64(sw - 1), 0},
		{float64(sw - 1), float64(sh - 1)},
		{0, float64(sh - 1)},
	}
	dstQuadF := [4][2]float64{
		{float64(dstQuad[0][0]), float64(dstQuad[0][1])},
		{float64(dstQuad[1][0]), float64(dstQuad[1][1])},
		{float64(dstQuad[2][0]), float64(dstQuad[2][1])},
		{float64(dstQuad[3][0]), float64(dstQuad[3][1])},
	}

	// We want, for each destination pixel, the source pixel that maps to it.
	// So we compute H: dst -> src directly.
	h, err := ComputeHomography(dstQuadF, srcQuad)
	if err != nil {
		return fmt.Errorf("failed to compute homography: %w", err)
	}

	// Bounding box of the destination quad, clipped to the destination canvas.
	dstBounds := dst.Bounds()
	minX := dstQuad[0][0]
	minY := dstQuad[0][1]
	maxX := dstQuad[0][0]
	maxY := dstQuad[0][1]
	for i := 1; i < 4; i++ {
		if dstQuad[i][0] < minX {
			minX = dstQuad[i][0]
		}
		if dstQuad[i][0] > maxX {
			maxX = dstQuad[i][0]
		}
		if dstQuad[i][1] < minY {
			minY = dstQuad[i][1]
		}
		if dstQuad[i][1] > maxY {
			maxY = dstQuad[i][1]
		}
	}
	if minX < dstBounds.Min.X {
		minX = dstBounds.Min.X
	}
	if minY < dstBounds.Min.Y {
		minY = dstBounds.Min.Y
	}
	if maxX > dstBounds.Max.X-1 {
		maxX = dstBounds.Max.X - 1
	}
	if maxY > dstBounds.Max.Y-1 {
		maxY = dstBounds.Max.Y - 1
	}

	// Precompute the edge functions for a point-in-quad test. The quad is
	// (tl, tr, br, bl); for each edge we need the signed "side" test.
	edges := [4][4]float64{
		{dstQuadF[0][0], dstQuadF[0][1], dstQuadF[1][0], dstQuadF[1][1]}, // tl -> tr
		{dstQuadF[1][0], dstQuadF[1][1], dstQuadF[2][0], dstQuadF[2][1]}, // tr -> br
		{dstQuadF[2][0], dstQuadF[2][1], dstQuadF[3][0], dstQuadF[3][1]}, // br -> bl
		{dstQuadF[3][0], dstQuadF[3][1], dstQuadF[0][0], dstQuadF[0][1]}, // bl -> tl
	}

	// Determine the winding by testing the quad centroid.
	cx := (dstQuadF[0][0] + dstQuadF[1][0] + dstQuadF[2][0] + dstQuadF[3][0]) / 4
	cy := (dstQuadF[0][1] + dstQuadF[1][1] + dstQuadF[2][1] + dstQuadF[3][1]) / 4
	centroidSigns := [4]float64{}
	for i := 0; i < 4; i++ {
		centroidSigns[i] = edgeSide(edges[i], cx, cy)
	}

	srcStride := src.Stride
	srcPix := src.Pix
	dstStride := dst.Stride
	dstPix := dst.Pix

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			// Point-in-quad test: every edge should return the same sign as the
			// centroid. We allow a tiny tolerance on the boundary.
			inside := true
			for i := 0; i < 4; i++ {
				side := edgeSide(edges[i], float64(x), float64(y))
				if side*centroidSigns[i] < -1e-6 {
					inside = false
					break
				}
			}
			if !inside {
				continue
			}

			// Map destination pixel to source coordinates.
			srcX, srcY := h.Apply(float64(x)+0.5, float64(y)+0.5)
			if srcX < 0 || srcY < 0 || srcX >= float64(sw) || srcY >= float64(sh) {
				continue
			}

			r, g, bl, a := bilinearSample(srcPix, srcStride, sw, sh, srcX, srcY)
			off := y*dstStride + x*4
			dstPix[off+0] = r
			dstPix[off+1] = g
			dstPix[off+2] = bl
			dstPix[off+3] = a
		}
	}
	return nil
}

// edgeSide returns the signed "side" of (px, py) with respect to the
// directed edge (ax, ay) -> (bx, by). Positive on one side, negative on the
// other, zero on the line itself.
func edgeSide(e [4]float64, px, py float64) float64 {
	ax, ay, bx, by := e[0], e[1], e[2], e[3]
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

// bilinearSample samples an NRGBA source at fractional coordinates using
// 2x2 bilinear filtering.
func bilinearSample(pix []uint8, stride, w, h int, fx, fy float64) (uint8, uint8, uint8, uint8) {
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	x1 := x0 + 1
	y1 := y0 + 1
	dx := fx - float64(x0)
	dy := fy - float64(y0)

	// Clamp to valid range. We already checked the outer bounds, but the
	// neighbour sample might still be one pixel past the edge.
	if x1 >= w {
		x1 = w - 1
	}
	if y1 >= h {
		y1 = h - 1
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	off00 := y0*stride + x0*4
	off10 := y0*stride + x1*4
	off01 := y1*stride + x0*4
	off11 := y1*stride + x1*4

	w00 := (1 - dx) * (1 - dy)
	w10 := dx * (1 - dy)
	w01 := (1 - dx) * dy
	w11 := dx * dy

	r := w00*float64(pix[off00+0]) + w10*float64(pix[off10+0]) + w01*float64(pix[off01+0]) + w11*float64(pix[off11+0])
	g := w00*float64(pix[off00+1]) + w10*float64(pix[off10+1]) + w01*float64(pix[off01+1]) + w11*float64(pix[off11+1])
	b := w00*float64(pix[off00+2]) + w10*float64(pix[off10+2]) + w01*float64(pix[off01+2]) + w11*float64(pix[off11+2])
	a := w00*float64(pix[off00+3]) + w10*float64(pix[off10+3]) + w01*float64(pix[off01+3]) + w11*float64(pix[off11+3])

	return uint8(r + 0.5), uint8(g + 0.5), uint8(b + 0.5), uint8(a + 0.5)
}
