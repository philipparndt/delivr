package rotato

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

// RenderWithCLI uses Rotato via UI automation to render a 3D mockup
func RenderWithCLI(rotatoFile, screenshotPath string, targetWidth, targetHeight int, verbose bool) (image.Image, error) {
	// Get absolute paths
	absRotato, err := filepath.Abs(rotatoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for rotato file: %w", err)
	}

	absScreenshot, err := filepath.Abs(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for screenshot: %w", err)
	}

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "rotato-export")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "export.png")

	if verbose {
		fmt.Printf("  Rotato: %s\n", filepath.Base(rotatoFile))
		fmt.Printf("  Screenshot: %s\n", filepath.Base(screenshotPath))
	}

	// Step 1: Copy image content (not file) to clipboard using Preview
	// This copies the actual image pixels, not a file reference
	copyScript := fmt.Sprintf(`
-- Open image in Preview and copy its contents
tell application "Preview"
	activate
	open POSIX file "%s"
	delay 1
end tell

tell application "System Events"
	tell process "Preview"
		set frontmost to true
		delay 0.3
		-- Select All
		keystroke "a" using {command down}
		delay 0.2
		-- Copy
		keystroke "c" using {command down}
		delay 0.3
	end tell
end tell

-- Close Preview
tell application "Preview"
	close window 1
end tell
`, absScreenshot)

	if verbose {
		fmt.Printf("  Copying image to clipboard...\n")
	}

	cmd := exec.Command("osascript", "-e", copyScript)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to copy image: %v\nOutput: %s", err, string(output))
	}

	time.Sleep(500 * time.Millisecond)

	// Step 2: Open Rotato file, select device, and paste
	rotatoScript := fmt.Sprintf(`
-- Open the Rotato file
tell application "Rotato"
	activate
	open POSIX file "%s"
end tell

delay 5

tell application "System Events"
	tell process "Rotato"
		set frontmost to true
		delay 1

		-- Select All to select the device
		keystroke "a" using {command down}
		delay 0.5

		-- Paste the image (should replace screen content)
		keystroke "v" using {command down}
		delay 1

		-- Export with Shift+E
		keystroke "e" using {shift down}
		delay 2

		-- In save dialog, go to folder
		keystroke "g" using {command down, shift down}
		delay 1

		keystroke "%s"
		delay 0.3
		keystroke return
		delay 0.8

		-- Set filename
		keystroke "a" using {command down}
		delay 0.2
		keystroke "export.png"
		delay 0.3

		-- Save
		keystroke return
		delay 2
	end tell
end tell
`, absRotato, tmpDir)

	if verbose {
		fmt.Printf("  Running Rotato automation...\n")
	}

	cmd = exec.Command("osascript", "-e", rotatoScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "assistive access") || strings.Contains(outputStr, "-25211") {
			return nil, fmt.Errorf(`accessibility permissions required

To fix: System Settings > Privacy & Security > Accessibility
Add and enable your terminal app, then restart it.`)
		}
		return nil, fmt.Errorf("automation failed: %v\nOutput: %s", err, outputStr)
	}

	// Wait for file to be written
	time.Sleep(1 * time.Second)

	// Check if file was created
	if _, statErr := os.Stat(tmpPath); statErr != nil {
		files, _ := filepath.Glob(filepath.Join(tmpDir, "*.png"))
		if len(files) > 0 {
			tmpPath = files[0]
		} else {
			return nil, fmt.Errorf("export file was not created at %s", tmpPath)
		}
	}

	// Load the exported image
	img, err := imaging.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Rotato export: %w", err)
	}

	if verbose {
		fmt.Printf("  Exported: %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	}

	// Scale if needed
	if targetWidth > 0 || targetHeight > 0 {
		img = scaleImage(img, targetWidth, targetHeight)
	}

	return img, nil
}

// RenderWithFrame composites a screenshot into a pre-rendered frame described
// by FrameMetadata. If the metadata reports an axis-aligned rectangle, a fast
// rect blit is used. Otherwise a full perspective warp is performed.
//
// The screen mask is the authoritative clip region: the screenshot is only
// drawn at pixels that were magenta in the original placeholder render. This
// keeps the device silhouette intact — the screenshot can never leak into
// the transparent background that Rotato leaves outside the device body.
//
// This is the fast path: no UI automation, no external process, just
// in-memory pixel work. Typical runtime is well under a second per
// screenshot, compared to ~20s for RenderWithCLI.
func RenderWithFrame(frame, mask *image.NRGBA, meta *FrameMetadata, screenshotPath string, targetWidth, targetHeight int) (image.Image, error) {
	screenshot, err := imaging.Open(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load screenshot: %w", err)
	}

	bounds := frame.Bounds()
	// Scratch canvas for the warped screenshot (before masking).
	scratch := image.NewNRGBA(bounds)

	if meta.IsRectangle {
		rectX := meta.RectangleRect[0]
		rectY := meta.RectangleRect[1]
		rectW := meta.RectangleRect[2]
		rectH := meta.RectangleRect[3]
		scaled := imaging.Resize(screenshot, rectW, rectH, imaging.Lanczos)
		draw.Draw(scratch, image.Rect(rectX, rectY, rectX+rectW, rectY+rectH), scaled, image.Point{}, draw.Src)
	} else {
		srcNRGBA := toNRGBA(screenshot)
		if err := WarpPerspective(srcNRGBA, scratch, meta.Corners); err != nil {
			return nil, fmt.Errorf("perspective warp failed: %w", err)
		}
	}

	// Apply the screen mask: keep scratch pixels wherever the mask is
	// opaque, zero them out everywhere else. This restricts the screenshot
	// to the exact device screen area.
	if mask != nil {
		maskPix := mask.Pix
		maskStride := mask.Stride
		scratchPix := scratch.Pix
		scratchStride := scratch.Stride
		h := bounds.Dy()
		w := bounds.Dx()
		for y := 0; y < h; y++ {
			mrow := maskPix[y*maskStride:]
			srow := scratchPix[y*scratchStride:]
			for x := 0; x < w; x++ {
				m := mrow[x*4+3] // mask alpha = opacity of screen area
				if m == 0 {
					off := x * 4
					srow[off+0] = 0
					srow[off+1] = 0
					srow[off+2] = 0
					srow[off+3] = 0
				} else if m != 255 {
					// Anti-aliased mask edge: scale the screenshot's alpha.
					off := x * 4
					srow[off+3] = uint8(int(srow[off+3]) * int(m) / 255)
				}
			}
		}
	}

	// Compose: masked screenshot underneath, frame on top.
	out := image.NewNRGBA(bounds)
	draw.Draw(out, bounds, scratch, bounds.Min, draw.Src)
	draw.Draw(out, bounds, frame, bounds.Min, draw.Over)

	var finalImg image.Image = out
	if targetWidth > 0 || targetHeight > 0 {
		finalImg = scaleImage(out, targetWidth, targetHeight)
	}
	return finalImg, nil
}

// DetectContentBounds finds the bounding box of non-transparent pixels in an image.
// alphaThreshold is the minimum alpha value (0-255) to consider as content.
// Use ~10-20 to include soft shadow edges, higher values for tighter cropping.
func DetectContentBounds(img image.Image, alphaThreshold uint8) image.Rectangle {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			// RGBA() returns 16-bit values, convert threshold to match
			if uint8(a>>8) > alphaThreshold {
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

	// If no content found, return original bounds
	if minX > maxX || minY > maxY {
		return bounds
	}

	// maxX and maxY are inclusive, but Rectangle.Max is exclusive
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// CropToContent crops an image to its non-transparent content area.
// alphaThreshold is the minimum alpha value (0-255) to consider as content.
// padding adds extra pixels around the detected content bounds.
func CropToContent(img image.Image, alphaThreshold uint8, padding int) image.Image {
	contentBounds := DetectContentBounds(img, alphaThreshold)

	// Apply padding
	paddedBounds := image.Rect(
		contentBounds.Min.X-padding,
		contentBounds.Min.Y-padding,
		contentBounds.Max.X+padding,
		contentBounds.Max.Y+padding,
	)

	// Clamp to image bounds
	imgBounds := img.Bounds()
	if paddedBounds.Min.X < imgBounds.Min.X {
		paddedBounds.Min.X = imgBounds.Min.X
	}
	if paddedBounds.Min.Y < imgBounds.Min.Y {
		paddedBounds.Min.Y = imgBounds.Min.Y
	}
	if paddedBounds.Max.X > imgBounds.Max.X {
		paddedBounds.Max.X = imgBounds.Max.X
	}
	if paddedBounds.Max.Y > imgBounds.Max.Y {
		paddedBounds.Max.Y = imgBounds.Max.Y
	}

	return imaging.Crop(img, paddedBounds)
}

// scaleImage scales an image maintaining aspect ratio
func scaleImage(img image.Image, targetWidth, targetHeight int) image.Image {
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	var newW, newH int
	if targetWidth == 0 {
		scale := float64(targetHeight) / float64(origH)
		newW = int(float64(origW) * scale)
		newH = targetHeight
	} else if targetHeight == 0 {
		scale := float64(targetWidth) / float64(origW)
		newW = targetWidth
		newH = int(float64(origH) * scale)
	} else {
		newW = targetWidth
		newH = targetHeight
	}

	return imaging.Resize(img, newW, newH, imaging.Lanczos)
}

// FindRotatoFiles finds all .rotato files in a directory.
// Resolves symlinks on the input directory so symlinked dirs work.
func FindRotatoFiles(dir string) ([]string, error) {
	// Resolve symlink on the root directory
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".rotato") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
