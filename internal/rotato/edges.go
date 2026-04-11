package rotato

import (
	"fmt"
	"math"
	"sort"
)

// findScreenCorners recovers the 4 true screen corners from the binary
// marker mask by detecting the 4 dominant straight edges of the mask's
// convex hull, fitting a line to each, and intersecting adjacent lines.
//
// How the dominant edges are found, without any assumptions about
// rotation or shape:
//
//  1. Extract hull candidates (leftmost and rightmost mask pixel per row).
//  2. Compute the convex hull (Andrew's monotone chain).
//  3. Walk the hull and compute each hull-edge direction (angle).
//  4. Starting at a point where the direction changes significantly,
//     group consecutive hull edges whose directions differ by less than
//     `angleTolerance` from the group's starting direction. Quantization
//     stair-steps on a rotated straight side all have very similar
//     directions so they end up in a single group; rounded-corner arcs
//     have a monotonically changing direction and break out of the group
//     after a handful of edges.
//  5. Take the 4 longest groups — these are the 4 straight sides.
//  6. For each group, fit a line via PCA on the hull vertices it covers.
//  7. Intersect consecutive fitted lines for the 4 true corners.
//  8. Relabel the corners as TL, TR, BR, BL.
//
// This works identically for axis-aligned rectangles, rotated rectangles
// and perspective trapezoids — every case has 4 straight sides that show
// up as 4 long direction-consistent hull segments.
func findScreenCorners(mask []uint8, width, height int) ([4][2]int, error) {
	candidates := extractHullCandidates(mask, width, height)
	if len(candidates) < 4 {
		return [4][2]int{}, fmt.Errorf("not enough mask pixels (%d) to compute a hull", len(candidates))
	}

	hull := convexHull(candidates)
	if len(hull) < 4 {
		return [4][2]int{}, fmt.Errorf("convex hull has %d vertices", len(hull))
	}

	const angleTolerance = 5 * math.Pi / 180 // 5°
	groups, err := groupHullEdgesByDirection(hull, angleTolerance)
	if err != nil {
		return [4][2]int{}, err
	}
	if len(groups) < 4 {
		return [4][2]int{}, fmt.Errorf("only %d direction groups found, need 4", len(groups))
	}

	// Pick the 4 longest groups (by cumulative edge length) — these are
	// the 4 straight sides. Restore cyclic order for corner intersection.
	sort.Slice(groups, func(i, j int) bool { return groups[i].totalLength > groups[j].totalLength })
	top4 := append([]edgeGroup(nil), groups[:4]...)
	sort.Slice(top4, func(i, j int) bool { return top4[i].startIdx < top4[j].startIdx })

	// Fit a line through each group's vertices using total least squares.
	var lines [4]lineEq
	for i, g := range top4 {
		verts := groupVertices(hull, g)
		if len(verts) < 2 {
			return [4][2]int{}, fmt.Errorf("group %d has too few vertices", i)
		}
		lines[i] = fitLineTLS(verts)
	}

	// Intersect consecutive lines for the 4 corners of the refined quad.
	var corners [4][2]float64
	for i := 0; i < 4; i++ {
		l1 := lines[i]
		l2 := lines[(i+1)%4]
		det := l1.a*l2.b - l2.a*l1.b
		if math.Abs(det) < 1e-9 {
			return [4][2]int{}, fmt.Errorf("fitted lines %d and %d are parallel", i, (i+1)%4)
		}
		x := (l1.c*l2.b - l2.c*l1.b) / det
		y := (l1.a*l2.c - l2.a*l1.c) / det
		corners[i] = [2]float64{x, y}
	}

	// Sanity-check: reject nonsense intersections far outside the frame.
	diag := math.Hypot(float64(width), float64(height))
	for _, c := range corners {
		if c[0] < -diag || c[1] < -diag || c[0] > 2*float64(width) || c[1] > 2*float64(height) {
			return [4][2]int{}, fmt.Errorf("corner (%.1f, %.1f) is far outside the frame", c[0], c[1])
		}
	}

	// Relabel as TL, TR, BR, BL by the usual min/max of x±y rule.
	return relabelCorners(corners), nil
}

type edgeGroup struct {
	// startIdx and endIdx bracket the hull edges in [startIdx, endIdx) in
	// cyclic order. endIdx may be < startIdx when the group wraps the hull.
	startIdx    int
	endIdx      int
	totalLength float64
	// startAngle is the direction of the first edge in the group; all
	// subsequent edges in the group are within angleTolerance of this.
	startAngle float64
}

// groupHullEdgesByDirection walks a convex hull and partitions its edges
// into runs whose directions match their group's starting direction
// within angleTolerance. The partition is cyclic: the first and last
// groups are merged if they share a direction.
func groupHullEdgesByDirection(hull [][2]int, angleTolerance float64) ([]edgeGroup, error) {
	n := len(hull)
	if n < 4 {
		return nil, fmt.Errorf("hull too small (%d vertices)", n)
	}

	angles := make([]float64, n)
	lengths := make([]float64, n)
	for i := 0; i < n; i++ {
		p := hull[i]
		q := hull[(i+1)%n]
		dx := float64(q[0] - p[0])
		dy := float64(q[1] - p[1])
		lengths[i] = math.Hypot(dx, dy)
		angles[i] = math.Atan2(dy, dx)
	}

	// Find a start index where the direction breaks significantly from
	// the previous edge — that way the wraparound doesn't split a group.
	startIdx := 0
	for i := 0; i < n; i++ {
		prev := (i - 1 + n) % n
		if angleAbsDiff(angles[prev], angles[i]) > angleTolerance {
			startIdx = i
			break
		}
	}

	var groups []edgeGroup
	curStart := startIdx
	curAngle := angles[startIdx]
	curLen := lengths[startIdx]
	curCount := 1

	closeGroup := func(endIdx int) {
		groups = append(groups, edgeGroup{
			startIdx:    curStart,
			endIdx:      endIdx,
			totalLength: curLen,
			startAngle:  curAngle,
		})
	}

	for k := 1; k < n; k++ {
		i := (startIdx + k) % n
		if angleAbsDiff(curAngle, angles[i]) < angleTolerance {
			curLen += lengths[i]
			curCount++
			continue
		}
		closeGroup(i)
		curStart = i
		curAngle = angles[i]
		curLen = lengths[i]
		curCount = 1
	}
	closeGroup(startIdx)

	// If the chosen start happens to sit in the middle of a long run,
	// the first and last groups may be two halves of the same side —
	// merge them.
	if len(groups) >= 2 {
		first := groups[0]
		last := groups[len(groups)-1]
		if angleAbsDiff(first.startAngle, last.startAngle) < angleTolerance {
			merged := edgeGroup{
				startIdx:    last.startIdx,
				endIdx:      first.endIdx,
				totalLength: first.totalLength + last.totalLength,
				startAngle:  last.startAngle,
			}
			groups = append([]edgeGroup{merged}, groups[1:len(groups)-1]...)
		}
	}
	return groups, nil
}

// groupVertices returns the hull vertices covered by a group (both
// endpoints of every edge in the group). The result may be non-trivial
// when the group wraps around the hull.
func groupVertices(hull [][2]int, g edgeGroup) [][2]float64 {
	n := len(hull)
	var out [][2]float64
	i := g.startIdx
	for {
		out = append(out, [2]float64{float64(hull[i][0]), float64(hull[i][1])})
		next := (i + 1) % n
		i = next
		if i == g.endIdx {
			// Include the end vertex (the endpoint of the last edge).
			out = append(out, [2]float64{float64(hull[i][0]), float64(hull[i][1])})
			break
		}
	}
	return out
}

// angleAbsDiff returns the absolute difference between two angles in
// radians, wrapped into [0, pi] (we don't care about direction, only the
// axis, because a straight line is the same in either direction).
func angleAbsDiff(a, b float64) float64 {
	d := math.Abs(a - b)
	for d > 2*math.Pi {
		d -= 2 * math.Pi
	}
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	// Also allow 180° flip — a line and its reverse are the same line.
	if d > math.Pi/2 {
		d = math.Pi - d
	}
	return d
}

// lineEq stores a line in the form a*x + b*y = c where (a, b) is the
// unit normal.
type lineEq struct {
	a, b, c float64
}

// fitLineTLS fits a line to a set of points using total least squares
// (PCA on the centred point cloud). The principal direction of maximum
// variance is the line direction; the orthogonal direction is the line
// normal. Passes through the centroid.
func fitLineTLS(points [][2]float64) lineEq {
	n := float64(len(points))
	var sx, sy float64
	for _, p := range points {
		sx += p[0]
		sy += p[1]
	}
	cx := sx / n
	cy := sy / n

	var sxx, syy, sxy float64
	for _, p := range points {
		dx := p[0] - cx
		dy := p[1] - cy
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
	}

	// Angle of the dominant (variance-maximising) direction.
	angle := 0.5 * math.Atan2(2*sxy, sxx-syy)
	dirX := math.Cos(angle)
	dirY := math.Sin(angle)

	// Normal is perpendicular to the direction.
	nx := -dirY
	ny := dirX
	c := nx*cx + ny*cy
	return lineEq{a: nx, b: ny, c: c}
}

// relabelCorners reorders 4 corner points so index 0 is TL (smallest
// x+y), 1 is TR (largest x-y), 2 is BR (largest x+y), 3 is BL.
func relabelCorners(c [4][2]float64) [4][2]int {
	const big = math.MaxFloat64
	minSum, maxSum := big, -big
	minDiff, maxDiff := big, -big
	var tl, tr, br, bl [2]float64
	for _, p := range c {
		s := p[0] + p[1]
		d := p[0] - p[1]
		if s < minSum {
			minSum = s
			tl = p
		}
		if s > maxSum {
			maxSum = s
			br = p
		}
		if d > maxDiff {
			maxDiff = d
			tr = p
		}
		if d < minDiff {
			minDiff = d
			bl = p
		}
	}
	return [4][2]int{
		{int(math.Round(tl[0])), int(math.Round(tl[1]))},
		{int(math.Round(tr[0])), int(math.Round(tr[1]))},
		{int(math.Round(br[0])), int(math.Round(br[1]))},
		{int(math.Round(bl[0])), int(math.Round(bl[1]))},
	}
}

// extractHullCandidates returns only the pixels that can possibly lie on
// the convex hull of the mask: the leftmost and rightmost mask pixel on
// each row. This reduces the candidate set from O(area) to O(height).
func extractHullCandidates(mask []uint8, width, height int) [][2]int {
	var out [][2]int
	for y := 0; y < height; y++ {
		row := y * width
		left := -1
		right := -1
		for x := 0; x < width; x++ {
			if mask[row+x] != 0 {
				if left == -1 {
					left = x
				}
				right = x
			}
		}
		if left == -1 {
			continue
		}
		out = append(out, [2]int{left, y})
		if right != left {
			out = append(out, [2]int{right, y})
		}
	}
	return out
}

// convexHull computes the convex hull using Andrew's monotone chain.
// Points are returned in counter-clockwise order. Strictly collinear
// points are dropped.
func convexHull(points [][2]int) [][2]int {
	n := len(points)
	if n < 3 {
		return points
	}
	pts := make([][2]int, n)
	copy(pts, points)
	sort.Slice(pts, func(i, j int) bool {
		if pts[i][0] != pts[j][0] {
			return pts[i][0] < pts[j][0]
		}
		return pts[i][1] < pts[j][1]
	})

	lower := make([][2]int, 0, n)
	for _, p := range pts {
		for len(lower) >= 2 && crossZ(lower[len(lower)-2], lower[len(lower)-1], p) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}

	upper := make([][2]int, 0, n)
	for i := n - 1; i >= 0; i-- {
		p := pts[i]
		for len(upper) >= 2 && crossZ(upper[len(upper)-2], upper[len(upper)-1], p) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}

	hull := make([][2]int, 0, len(lower)+len(upper)-2)
	hull = append(hull, lower[:len(lower)-1]...)
	hull = append(hull, upper[:len(upper)-1]...)
	return hull
}

// crossZ returns the z-component of the cross product of (a-o) and (b-o).
func crossZ(o, a, b [2]int) int64 {
	return int64(a[0]-o[0])*int64(b[1]-o[1]) -
		int64(a[1]-o[1])*int64(b[0]-o[0])
}
