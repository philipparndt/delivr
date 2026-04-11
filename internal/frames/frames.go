package frames

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"

	"github.com/disintegration/imaging"

	"github.com/philipparndt/delivr/internal/rotato"
)

// ScreenRegion describes a detected rectangular screen area in a bezel image.
type ScreenRegion struct {
	X, Y, W, H int
}

// DetectScreenRegion finds the transparent screen hole in a bezel PNG.
// Apple's bezels have transparency in two places: the screen hole AND the
// outside of the device. We flood-fill from the image edges to mark "outside"
// transparent pixels, then any remaining transparent pixels are the screen.
func DetectScreenRegion(img image.Image) (ScreenRegion, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Build an "outside" mask via flood-fill from edges.
	// A pixel is "outside" if it's transparent and reachable from the border.
	outside := make([]bool, w*h)
	isTransparent := func(x, y int) bool {
		_, _, _, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
		return a == 0
	}

	// BFS flood-fill from all transparent border pixels
	type point struct{ x, y int }
	queue := make([]point, 0, w*2+h*2)

	enqueue := func(x, y int) {
		idx := y*w + x
		if x >= 0 && x < w && y >= 0 && y < h && !outside[idx] && isTransparent(x, y) {
			outside[idx] = true
			queue = append(queue, point{x, y})
		}
	}

	// Seed from all 4 borders
	for x := 0; x < w; x++ {
		enqueue(x, 0)
		enqueue(x, h-1)
	}
	for y := 0; y < h; y++ {
		enqueue(0, y)
		enqueue(w-1, y)
	}

	// BFS
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		enqueue(p.x-1, p.y)
		enqueue(p.x+1, p.y)
		enqueue(p.x, p.y-1)
		enqueue(p.x, p.y+1)
	}

	// Now find bounding box of transparent pixels that are NOT outside (= screen)
	minX, minY := w, h
	maxX, maxY := -1, -1
	found := false

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if isTransparent(x, y) && !outside[y*w+x] {
				found = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if !found {
		return ScreenRegion{}, fmt.Errorf("no screen region found — the bezel may not have a transparent screen hole")
	}

	return ScreenRegion{
		X: minX + bounds.Min.X,
		Y: minY + bounds.Min.Y,
		W: maxX - minX + 1,
		H: maxY - minY + 1,
	}, nil
}

// GenerateMask creates a mask image from a bezel PNG and the detected screen region.
// The mask is opaque (white) only inside the screen area and transparent everywhere
// else. Uses flood-fill from the screen center through fully-transparent pixels
// to find the screen hole, then marks those pixels as opaque in the mask.
func GenerateMask(img image.Image, region ScreenRegion) *image.NRGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	mask := image.NewNRGBA(bounds)

	// Flood-fill from the screen center through fully transparent pixels only.
	// This avoids leaking through semi-transparent frame edge pixels.
	screen := make([]bool, w*h)
	alphaAt := func(x, y int) uint8 {
		_, _, _, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
		return uint8(a >> 8)
	}

	type point struct{ x, y int }
	queue := make([]point, 0, region.W*region.H)

	cx := region.X - bounds.Min.X + region.W/2
	cy := region.Y - bounds.Min.Y + region.H/2

	enqueue := func(x, y int) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		idx := y*w + x
		if screen[idx] {
			return
		}
		// Only flood through fully transparent pixels
		if alphaAt(x, y) != 0 {
			return
		}
		screen[idx] = true
		queue = append(queue, point{x, y})
	}

	enqueue(cx, cy)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		enqueue(p.x-1, p.y)
		enqueue(p.x+1, p.y)
		enqueue(p.x, p.y-1)
		enqueue(p.x, p.y+1)
	}

	// Build mask: fully opaque where screen is transparent
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if screen[y*w+x] {
				mask.SetNRGBA(x+bounds.Min.X, y+bounds.Min.Y,
					color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}

	return mask
}

// GenerateTemplate converts a bezel PNG into a delivr template set (frame PNG +
// mask PNG + metadata JSON). The bezel must have a transparent screen hole.
func GenerateTemplate(bezelPath, outputDir string, verbose bool) (string, string, string, error) {
	img, err := imaging.Open(bezelPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open bezel image: %w", err)
	}

	// Convert to NRGBA for consistent pixel access
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}

	// Detect screen region
	region, err := DetectScreenRegion(nrgba)
	if err != nil {
		return "", "", "", fmt.Errorf("screen detection failed: %w", err)
	}

	if verbose {
		fmt.Printf("  Screen region: %dx%d at (%d, %d)\n", region.W, region.H, region.X, region.Y)
	}

	// Generate mask (only opaque within the screen region)
	mask := GenerateMask(nrgba, region)

	// Build metadata
	meta := &rotato.FrameMetadata{
		FrameWidth:  bounds.Dx(),
		FrameHeight: bounds.Dy(),
		Corners: [4][2]int{
			{region.X, region.Y},                       // TL
			{region.X + region.W - 1, region.Y},        // TR
			{region.X + region.W - 1, region.Y + region.H - 1}, // BR
			{region.X, region.Y + region.H - 1},        // BL
		},
		IsRectangle:   true,
		RectangleRect: [4]int{region.X, region.Y, region.W, region.H},
		Source: rotato.FrameSource{
			RotatoTemplate:    filepath.Base(bezelPath),
			PlaceholderWidth:  region.W,
			PlaceholderHeight: region.H,
		},
	}

	// Save using existing SaveFrame (the last param is used to derive the stem)
	return rotato.SaveFrame(nrgba, mask, meta, outputDir, bezelPath)
}
