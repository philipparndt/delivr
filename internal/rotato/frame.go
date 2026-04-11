package rotato

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// MarkerColor is the placeholder color used to mark the screen region in the
// pre-rendered Rotato frame. Pure magenta is chosen because it virtually never
// appears in real UI screenshots, making detection unambiguous.
var MarkerColor = color.NRGBA{R: 255, G: 0, B: 255, A: 255}

// spillThreshold is the minimum "magenta content" we consider meaningful.
// Pixels below this threshold are treated as "not magenta" by both detection
// and despill so the two stay in sync. The threshold also suppresses
// bezel/background pixels that pick up a faint magenta cast from Rotato's
// specular highlights and screen glow.
const spillThreshold = 25

// FrameMetadata describes a pre-rendered Rotato frame: the PNG file has a
// transparent hole where the device screen sits, and Corners contains the 4
// destination points in the frame image coordinate system, in TL, TR, BR, BL
// order. For axis-aligned rectangles IsRectangle is true and callers may use
// a fast blit path instead of a perspective warp.
//
// MaskPath points to a single-channel PNG (stored as L8) that is opaque
// exactly where the screen area is. It is used to clip the warped screenshot
// so the device silhouette is preserved and the screenshot cannot leak into
// areas that Rotato left transparent outside the device body.
type FrameMetadata struct {
	FramePath     string      `json:"frame_path"`
	MaskPath      string      `json:"mask_path"`
	FrameWidth    int         `json:"frame_width"`
	FrameHeight   int         `json:"frame_height"`
	Corners       [4][2]int   `json:"corners"`
	IsRectangle   bool        `json:"is_rectangle"`
	RectangleRect [4]int      `json:"rectangle_rect,omitempty"` // [x, y, w, h] when IsRectangle
	Source        FrameSource `json:"source"`
}

// FrameSource records what the frame was generated from so it can be
// regenerated deterministically.
type FrameSource struct {
	RotatoTemplate   string `json:"rotato_template"`
	PlaceholderWidth int    `json:"placeholder_width"`
	PlaceholderHeight int   `json:"placeholder_height"`
}

// GeneratePlaceholder writes a solid magenta PNG of the requested size.
// It is used as input to the Rotato UI automation so we get back a rendered
// frame with a clearly-identifiable screen region.
func GeneratePlaceholder(width, height int, path string) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("placeholder dimensions must be positive, got %dx%d", width, height)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	// Fill with magenta. Using a tight loop over the pixel buffer is faster
	// than image.Draw for this case.
	pix := img.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = MarkerColor.R
		pix[i+1] = MarkerColor.G
		pix[i+2] = MarkerColor.B
		pix[i+3] = MarkerColor.A
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create placeholder directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create placeholder file: %w", err)
	}
	defer f.Close()
	return png.Encode(f, img)
}

// magentaSpill returns the amount of magenta contribution in a pixel,
// computed as min(R, B) - G. For pure magenta (255, 0, 255) this is 255.
// For any grey color it is 0. For bezel pixels with slight specular
// reflections of the magenta screen it is a small positive number.
//
// This is the single unified magenta-ness metric used by both the detection
// pass (which determines the screen geometry) and the despill pass (which
// removes magenta from the saved frame). Using the same metric in both
// places guarantees the detected geometry and the transparent hole in the
// frame PNG match exactly.
func magentaSpill(r, g, b, a uint8) int {
	if a < 200 {
		return 0
	}
	minRB := int(r)
	if int(b) < minRB {
		minRB = int(b)
	}
	spill := minRB - int(g)
	if spill < 0 {
		return 0
	}
	return spill
}

// isMarker reports whether a pixel has enough magenta to be considered part
// of the screen region. Uses the same threshold as the despill.
func isMarker(r, g, b, a uint8) bool {
	return magentaSpill(r, g, b, a) >= spillThreshold
}

// markerStats aggregates everything we need to reconstruct the screen
// geometry and the screen mask from a single pass over the rendered frame.
type markerStats struct {
	count int64
	// Tight axis-aligned bounding box.
	minX, minY int
	maxX, maxY int
	// 4-extrema via min/max of x±y. Works for axis-aligned rectangles,
	// in-plane rotations and perspective trapezoids.
	tl, tr, br, bl [2]int
	// Binary mask: mask[y*width + x] == 1 when the pixel is magenta.
	mask   []uint8
	width  int
	height int
}

// collectMarkerStats scans the image once and returns all the aggregated
// stats plus a single-channel mask image that is the authoritative source
// of "is this pixel part of the screen area" for both downstream geometry
// and for clipping the screenshot during compositing.
func collectMarkerStats(rgba *image.NRGBA) (markerStats, error) {
	bounds := rgba.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return markerStats{}, fmt.Errorf("frame image is empty")
	}

	const big = math.MaxInt32
	stats := markerStats{
		minX:   big,
		minY:   big,
		maxX:   -big,
		maxY:   -big,
		width:  w,
		height: h,
		mask:   make([]uint8, w*h),
	}

	minSum, maxSum := big, -big
	minDiff, maxDiff := big, -big

	pix := rgba.Pix
	stride := rgba.Stride

	for y := 0; y < h; y++ {
		row := pix[y*stride:]
		for x := 0; x < w; x++ {
			off := x * 4
			r, g, b, a := row[off], row[off+1], row[off+2], row[off+3]
			if !isMarker(r, g, b, a) {
				continue
			}
			stats.count++
			stats.mask[y*w+x] = 255
			if x < stats.minX {
				stats.minX = x
			}
			if x > stats.maxX {
				stats.maxX = x
			}
			if y < stats.minY {
				stats.minY = y
			}
			if y > stats.maxY {
				stats.maxY = y
			}
			s := x + y
			d := x - y
			if s < minSum {
				minSum = s
				stats.tl = [2]int{x, y}
			}
			if s > maxSum {
				maxSum = s
				stats.br = [2]int{x, y}
			}
			if d > maxDiff {
				maxDiff = d
				stats.tr = [2]int{x, y}
			}
			if d < minDiff {
				minDiff = d
				stats.bl = [2]int{x, y}
			}
		}
	}

	if stats.count == 0 {
		return stats, fmt.Errorf("no marker pixels found; placeholder may not have been accepted by Rotato")
	}
	return stats, nil
}

// quadScaleFactor is how much to enlarge the detected quad relative to its
// centroid. The detected extrema sit inside the rounded-corner arcs by
// roughly 0.29×r where r is the corner radius. With the mask clipping every
// pixel during composite, any overshoot is harmless but any undershoot
// leaves gaps at the edges of the screen.
const quadScaleFactor = 1.03

// expandQuadFromCentroid grows a quad uniformly around its centroid by the
// given factor. This preserves the shape (axis-aligned rectangle, rotated
// rectangle, or perspective trapezoid) so the warp keeps the correct
// proportions while covering the rounded-corner slack.
func expandQuadFromCentroid(quad [4][2]int, factor float64) [4][2]int {
	var cx, cy float64
	for i := 0; i < 4; i++ {
		cx += float64(quad[i][0])
		cy += float64(quad[i][1])
	}
	cx /= 4
	cy /= 4

	var out [4][2]int
	for i := 0; i < 4; i++ {
		dx := float64(quad[i][0]) - cx
		dy := float64(quad[i][1]) - cy
		out[i] = [2]int{
			int(math.Round(cx + dx*factor)),
			int(math.Round(cy + dy*factor)),
		}
	}
	return out
}

// DetectFrame analyses a Rotato-rendered frame with a magenta screen region
// and returns the cleaned frame (magenta -> transparent), a screen-area
// mask (opaque where the screen is, transparent everywhere else), and the
// metadata describing where the screen sits.
func DetectFrame(rendered image.Image, rotatoTemplate string, placeholderW, placeholderH int) (*image.NRGBA, *image.NRGBA, *FrameMetadata, error) {
	rgba := toNRGBA(rendered)

	stats, err := collectMarkerStats(rgba)
	if err != nil {
		return nil, nil, nil, err
	}

	// Sanity-check the detected area is plausible.
	bounds := rendered.Bounds()
	totalPixels := int64(bounds.Dx()) * int64(bounds.Dy())
	if stats.count*1000 < totalPixels {
		return nil, nil, nil, fmt.Errorf("marker region is suspiciously small (%d pixels out of %d)", stats.count, totalPixels)
	}

	cleaned := replaceMarkerWithTransparent(rgba)
	mask := maskImageFromStats(stats)

	// Punch the screen area fully transparent in the cleaned frame.
	// Rotato bakes a semi-transparent gloss/reflection highlight on top of
	// the magenta screen. The continuous despill removes the magenta
	// component but leaves the gloss's own color intact, which then
	// multiplies over the screenshot at composite time — subtly darkening
	// bright/light screenshots and tinting them. The mask is the
	// authoritative "this is the screen" region, so inside the mask the
	// frame must contribute nothing. Outside the mask the despilled soft
	// edges remain untouched and still blend the device bezel cleanly.
	clearFrameInsideMask(cleaned, stats.mask, stats.width, stats.height)

	// Recover the 4 true screen corners by finding the 4 dominant
	// straight edges of the mask's convex hull, fitting a line through
	// each, and intersecting adjacent lines. This works for axis-aligned
	// rectangles, rotated rectangles, and perspective trapezoids alike,
	// and it recovers the true edge orientation rather than relying on
	// the image-aligned 4-extrema of the mask.
	corners, cerr := findScreenCorners(stats.mask, stats.width, stats.height)
	if cerr != nil {
		// Fallback: raw 4-extrema with a small uniform expansion. Used
		// only if the hull analysis fails on a pathological mask.
		rawCorners := [4][2]int{stats.tl, stats.tr, stats.br, stats.bl}
		corners = expandQuadFromCentroid(rawCorners, quadScaleFactor)
	}

	// Detect axis-aligned rectangles (front views) so they can use the
	// fast rect-blit path instead of a full perspective warp.
	isRect := false
	var rect [4]int
	if looksAxisAligned(corners) {
		isRect = true
		minX, minY, maxX, maxY := cornersBBox(corners)
		corners = [4][2]int{
			{minX, minY},
			{maxX, minY},
			{maxX, maxY},
			{minX, maxY},
		}
		rect = [4]int{minX, minY, maxX - minX + 1, maxY - minY + 1}
	}

	meta := &FrameMetadata{
		FrameWidth:    bounds.Dx(),
		FrameHeight:   bounds.Dy(),
		Corners:       corners,
		IsRectangle:   isRect,
		RectangleRect: rect,
		Source: FrameSource{
			RotatoTemplate:    rotatoTemplate,
			PlaceholderWidth:  placeholderW,
			PlaceholderHeight: placeholderH,
		},
	}

	return cleaned, mask, meta, nil
}

// looksAxisAligned reports whether the 4 corners describe an axis-aligned
// rectangle within a small tolerance (fraction of a pixel-ish). The hull-
// based corner detection reports the true geometric corners, so this is a
// strict test: if the top edge isn't horizontal to within a few pixels, it
// is a rotated rectangle or trapezoid.
func looksAxisAligned(c [4][2]int) bool {
	// Tolerance is absolute — the hull detection is accurate to a pixel
	// or two even on large canvases.
	const tol = 3
	// TL[y] ≈ TR[y], BL[y] ≈ BR[y], TL[x] ≈ BL[x], TR[x] ≈ BR[x].
	tl, tr, br, bl := c[0], c[1], c[2], c[3]
	diffs := [4]int{
		tl[1] - tr[1],
		bl[1] - br[1],
		tl[0] - bl[0],
		tr[0] - br[0],
	}
	for _, d := range diffs {
		if d < 0 {
			d = -d
		}
		if d > tol {
			return false
		}
	}
	return true
}

// cornersBBox returns the axis-aligned bounding box of the 4 corners as
// (minX, minY, maxX, maxY).
func cornersBBox(c [4][2]int) (int, int, int, int) {
	minX, minY := c[0][0], c[0][1]
	maxX, maxY := c[0][0], c[0][1]
	for i := 1; i < 4; i++ {
		if c[i][0] < minX {
			minX = c[i][0]
		}
		if c[i][0] > maxX {
			maxX = c[i][0]
		}
		if c[i][1] < minY {
			minY = c[i][1]
		}
		if c[i][1] > maxY {
			maxY = c[i][1]
		}
	}
	return minX, minY, maxX, maxY
}

// maskImageFromStats turns the binary mask inside markerStats into an NRGBA
// image where white+opaque marks the screen area and black+transparent
// marks everything else. Stored as RGBA rather than single-channel to keep
// loading trivial; png.Encode will compress it efficiently anyway.
func maskImageFromStats(s markerStats) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, s.width, s.height))
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			if s.mask[y*s.width+x] != 0 {
				off := y*out.Stride + x*4
				out.Pix[off+0] = 255
				out.Pix[off+1] = 255
				out.Pix[off+2] = 255
				out.Pix[off+3] = 255
			}
		}
	}
	return out
}

// replaceMarkerWithTransparent returns a copy of img where magenta has been
// removed via a continuous despill:
//
//   - Fully magenta pixels become fully transparent.
//   - Partially magenta pixels (anti-aliased edges around the bezel and
//     dynamic island) have their alpha reduced proportionally and their
//     red/blue channels pulled down to the green channel, which removes the
//     magenta tint and leaves the underlying device color.
//   - Non-magenta pixels are kept unchanged.
//
// This relies on the fact that the placeholder is pure (255, 0, 255), so any
// red+blue excess over green at a pixel is exactly the magenta contribution
// contributed by the placeholder during anti-aliased blending.
//
// Pixels with magenta spill below `spillThreshold` are left untouched. This
// keeps bezel pixels with faint specular magenta bleed fully opaque, so the
// device silhouette stays intact.
func replaceMarkerWithTransparent(src *image.NRGBA) *image.NRGBA {
	bounds := src.Bounds()
	out := image.NewNRGBA(bounds)
	copy(out.Pix, src.Pix)

	stride := out.Stride
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := out.Pix[y*stride:]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := x * 4
			r := int(row[off+0])
			g := int(row[off+1])
			b := int(row[off+2])
			a := int(row[off+3])

			minRB := r
			if b < minRB {
				minRB = b
			}
			spill := minRB - g
			if spill < spillThreshold {
				continue
			}
			if spill > 255 {
				spill = 255
			}

			row[off+0] = uint8(r - spill)
			// G is unchanged.
			row[off+2] = uint8(b - spill)
			row[off+3] = uint8(a * (255 - spill) / 255)
		}
	}
	return out
}

// clearFrameInsideMask zeroes the alpha of every frame pixel whose
// corresponding mask bit is set. The mask is the binary screen region
// detected from the magenta placeholder, so this removes any lingering
// contribution from Rotato's gloss/reflection overlay in the screen area.
func clearFrameInsideMask(frame *image.NRGBA, mask []uint8, w, h int) {
	stride := frame.Stride
	pix := frame.Pix
	for y := 0; y < h; y++ {
		row := pix[y*stride:]
		mrow := mask[y*w:]
		for x := 0; x < w; x++ {
			if mrow[x] == 0 {
				continue
			}
			off := x * 4
			row[off+0] = 0
			row[off+1] = 0
			row[off+2] = 0
			row[off+3] = 0
		}
	}
}

// toNRGBA converts any image to *image.NRGBA for predictable pixel access.
// If the input already is an NRGBA it is returned directly (zero copy).
func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			off := (y-bounds.Min.Y)*out.Stride + (x-bounds.Min.X)*4
			out.Pix[off+0] = uint8(r >> 8)
			out.Pix[off+1] = uint8(g >> 8)
			out.Pix[off+2] = uint8(b >> 8)
			out.Pix[off+3] = uint8(a >> 8)
		}
	}
	return out
}

// SaveDebugFrame writes a diagnostic PNG showing the detected corners as
// bright outlines on top of the despilled frame. It also renders a
// checkerboard background where the frame is transparent so the shape of
// the hole is easy to inspect.
func SaveDebugFrame(frame *image.NRGBA, meta *FrameMetadata, outputDir, rotatoTemplate string) (string, error) {
	bounds := frame.Bounds()
	dbg := image.NewNRGBA(bounds)

	// Checkerboard background.
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := dbg.Pix[y*dbg.Stride:]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := x * 4
			tile := ((x/32)+(y/32))%2 == 0
			if tile {
				row[off+0] = 200
				row[off+1] = 200
				row[off+2] = 200
			} else {
				row[off+0] = 150
				row[off+1] = 150
				row[off+2] = 150
			}
			row[off+3] = 255
		}
	}
	// Composite the frame on top so transparent pixels reveal the checkerboard.
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := y*frame.Stride + x*4
			fr, fg, fb, fa := frame.Pix[off], frame.Pix[off+1], frame.Pix[off+2], frame.Pix[off+3]
			if fa == 0 {
				continue
			}
			doff := y*dbg.Stride + x*4
			br, bg, bb := dbg.Pix[doff], dbg.Pix[doff+1], dbg.Pix[doff+2]
			a := int(fa)
			dbg.Pix[doff+0] = uint8((int(fr)*a + int(br)*(255-a)) / 255)
			dbg.Pix[doff+1] = uint8((int(fg)*a + int(bg)*(255-a)) / 255)
			dbg.Pix[doff+2] = uint8((int(fb)*a + int(bb)*(255-a)) / 255)
			dbg.Pix[doff+3] = 255
		}
	}
	// Draw the detected corners as bright lime circles and the quad edges
	// as lime lines.
	lime := color.NRGBA{R: 50, G: 255, B: 80, A: 255}
	for i := 0; i < 4; i++ {
		p := meta.Corners[i]
		drawCross(dbg, p[0], p[1], 25, lime)
		q := meta.Corners[(i+1)%4]
		drawLine(dbg, p[0], p[1], q[0], q[1], lime)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	stem := strings.TrimSuffix(filepath.Base(rotatoTemplate), filepath.Ext(rotatoTemplate))
	path := filepath.Join(outputDir, stem+".debug.png")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, dbg); err != nil {
		return "", err
	}
	return path, nil
}

func drawCross(img *image.NRGBA, cx, cy, size int, col color.NRGBA) {
	for d := -size; d <= size; d++ {
		setPixelSafe(img, cx+d, cy, col)
		setPixelSafe(img, cx, cy+d, col)
	}
}

func drawLine(img *image.NRGBA, x0, y0, x1, y1 int, col color.NRGBA) {
	dx := x1 - x0
	dy := y1 - y0
	steps := dx
	if dy > steps {
		steps = dy
	}
	if -dx > steps {
		steps = -dx
	}
	if -dy > steps {
		steps = -dy
	}
	if steps == 0 {
		setPixelSafe(img, x0, y0, col)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(math.Round(float64(x0) + t*float64(dx)))
		y := int(math.Round(float64(y0) + t*float64(dy)))
		// 3-pixel thick line
		for oy := -1; oy <= 1; oy++ {
			for ox := -1; ox <= 1; ox++ {
				setPixelSafe(img, x+ox, y+oy, col)
			}
		}
	}
}

func setPixelSafe(img *image.NRGBA, x, y int, col color.NRGBA) {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}
	off := y*img.Stride + x*4
	img.Pix[off+0] = col.R
	img.Pix[off+1] = col.G
	img.Pix[off+2] = col.B
	img.Pix[off+3] = col.A
}

// SaveFrame writes the cleaned frame PNG, the screen mask PNG, and the
// metadata JSON. All three files are placed in outputDir with names derived
// from the Rotato template stem.
func SaveFrame(frame, mask *image.NRGBA, meta *FrameMetadata, outputDir, rotatoTemplate string) (string, string, string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create output directory: %w", err)
	}

	stem := strings.TrimSuffix(filepath.Base(rotatoTemplate), filepath.Ext(rotatoTemplate))
	pngPath := filepath.Join(outputDir, stem+".frame.png")
	maskPath := filepath.Join(outputDir, stem+".frame.mask.png")
	jsonPath := filepath.Join(outputDir, stem+".frame.json")

	if err := writePNG(pngPath, frame); err != nil {
		return "", "", "", fmt.Errorf("failed to write frame png: %w", err)
	}
	if err := writePNG(maskPath, mask); err != nil {
		return "", "", "", fmt.Errorf("failed to write mask png: %w", err)
	}

	// Paths are stored relative to the JSON file for portability.
	meta.FramePath = filepath.Base(pngPath)
	meta.MaskPath = filepath.Base(maskPath)
	jsonBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		return "", "", "", fmt.Errorf("failed to write metadata: %w", err)
	}
	return pngPath, maskPath, jsonPath, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// LoadFrame reads a frame metadata JSON file, the cleaned frame PNG, and
// the screen mask PNG.
func LoadFrame(jsonPath string) (*image.NRGBA, *image.NRGBA, *FrameMetadata, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, nil, nil, err
	}
	var meta FrameMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse frame metadata: %w", err)
	}
	frame, err := loadPNGRel(jsonPath, meta.FramePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load frame png: %w", err)
	}
	var mask *image.NRGBA
	if meta.MaskPath != "" {
		mask, err = loadPNGRel(jsonPath, meta.MaskPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to load mask png: %w", err)
		}
	}
	return frame, mask, &meta, nil
}

func loadPNGRel(jsonPath, rel string) (*image.NRGBA, error) {
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(filepath.Dir(jsonPath), p)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return toNRGBA(img), nil
}

// FrameJSONForTemplate returns the expected frame metadata path next to a
// given .rotato template. It does not check whether the file exists.
func FrameJSONForTemplate(rotatoTemplate, framesDir string) string {
	stem := strings.TrimSuffix(filepath.Base(rotatoTemplate), filepath.Ext(rotatoTemplate))
	return filepath.Join(framesDir, stem+".frame.json")
}
