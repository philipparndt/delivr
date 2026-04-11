package render

import (
	"fmt"
	"image"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/rotato"
)

// RenderDevice renders the device screenshot
func RenderDevice(dc *gg.Context, deviceCfg *config.DeviceImage, screenshotsDir, deviceName string) error {
	// Resolve source path with device placeholder
	sourcePath := strings.ReplaceAll(deviceCfg.Source, "{device}", deviceName)
	fullPath := screenshotsDir + "/" + sourcePath

	var img image.Image
	var err error

	if deviceCfg.Template != "" {
		// Template mode: load frame JSON and composite screenshot
		templatePath := screenshotsDir + "/" + deviceCfg.Template
		frame, mask, meta, loadErr := rotato.LoadFrame(templatePath)
		if loadErr != nil {
			return fmt.Errorf("failed to load device template %s: %w", templatePath, loadErr)
		}
		img, err = rotato.RenderWithFrame(frame, mask, meta, fullPath, deviceCfg.Width, deviceCfg.Height)
		if err != nil {
			return fmt.Errorf("failed to render with device template: %w", err)
		}
	} else {
		// Flat mode: load screenshot directly
		img, err = loadAndScaleImage(fullPath, deviceCfg.Width, deviceCfg.Height)
		if err != nil {
			return fmt.Errorf("failed to load device image: %w", err)
		}
	}

	// Auto-crop to content bounds if enabled
	if deviceCfg.AutoCrop {
		threshold := uint8(deviceCfg.CropThreshold)
		if threshold == 0 {
			threshold = 10 // Default: include soft shadow edges
		}
		padding := deviceCfg.CropPadding

		img = rotato.CropToContent(img, threshold, padding)
	}

	// Scale after cropping if dimensions specified
	if deviceCfg.AutoCrop && (deviceCfg.Width > 0 || deviceCfg.Height > 0) {
		img = scaleImageToFit(img, deviceCfg.Width, deviceCfg.Height)
	}

	// Calculate position (centered + offset)
	imgW := img.Bounds().Dx()
	x := (float64(dc.Width())-float64(imgW))/2 + deviceCfg.X
	y := deviceCfg.Y

	dc.DrawImage(img, int(x), int(y))

	return nil
}

// scaleImageToFit scales an image to fit within target dimensions, maintaining aspect ratio
func scaleImageToFit(img image.Image, targetWidth, targetHeight int) image.Image {
	if targetWidth == 0 && targetHeight == 0 {
		return img
	}

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

// loadAndScaleImage loads an image and scales it to the target dimensions
func loadAndScaleImage(path string, targetWidth, targetHeight int) (image.Image, error) {
	img, err := imaging.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image %s: %w", path, err)
	}

	// If both dimensions are 0, return original
	if targetWidth == 0 && targetHeight == 0 {
		return img, nil
	}

	// Calculate dimensions maintaining aspect ratio
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	var newW, newH int
	if targetWidth == 0 {
		// Calculate width from height
		scale := float64(targetHeight) / float64(origH)
		newW = int(float64(origW) * scale)
		newH = targetHeight
	} else if targetHeight == 0 {
		// Calculate height from width
		scale := float64(targetWidth) / float64(origW)
		newW = targetWidth
		newH = int(float64(origH) * scale)
	} else {
		newW = targetWidth
		newH = targetHeight
	}

	// Scale using Lanczos for high quality
	return imaging.Resize(img, newW, newH, imaging.Lanczos), nil
}
