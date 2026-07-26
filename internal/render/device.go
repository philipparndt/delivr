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
	img, err := PrepareDeviceImage(deviceCfg, screenshotsDir, deviceName)
	if err != nil {
		return err
	}
	DrawDeviceImage(dc, deviceCfg, img)
	return nil
}

// DeviceSourcePath resolves a device config's source to a file path, expanding
// the {device} placeholder the same way the renderer does.
func DeviceSourcePath(deviceCfg *config.DeviceImage, screenshotsDir, deviceName string) string {
	return screenshotsDir + "/" + strings.ReplaceAll(deviceCfg.Source, "{device}", deviceName)
}

// DeviceImageInfo records what PrepareDeviceImage actually did to the source,
// so a caller reasoning about where things ended up can use measurements
// instead of predictions.
//
// The crop is the step worth reporting. It happens *after* the first scale, so
// its rectangle — and the crop_padding inside it — are in scaled pixels, not in
// the source's own. Anything mapping a source coordinate onto the canvas has to
// go through this rectangle to get the right answer.
type DeviceImageInfo struct {
	// Stage is the image size after the first scale, before any crop.
	Stage image.Point
	// Crop is the rectangle cut out of the staged image. The whole staged
	// image when auto_crop is off.
	Crop image.Rectangle
	// Final is the size of the image that gets drawn.
	Final image.Point
}

// PrepareDeviceImage produces the image RenderDevice will draw: the screenshot
// composited into its frame, cropped to content, and scaled. Everything except
// deciding where to put it.
//
// It depends on the source, the frame and the target size, but not on x or y,
// so a caller redrawing the same device at different positions can cache the
// result. See CompositeDeviceImage and SizeDeviceImage for the finer split.
func PrepareDeviceImage(deviceCfg *config.DeviceImage, screenshotsDir, deviceName string) (image.Image, error) {
	img, _, err := PrepareDeviceImageInfo(deviceCfg, screenshotsDir, deviceName)
	return img, err
}

// PrepareDeviceImageInfo is PrepareDeviceImage, also reporting the intermediate
// sizes and the crop it took.
func PrepareDeviceImageInfo(deviceCfg *config.DeviceImage, screenshotsDir, deviceName string) (image.Image, DeviceImageInfo, error) {
	composite, err := CompositeDeviceImage(deviceCfg, screenshotsDir, deviceName)
	if err != nil {
		return nil, DeviceImageInfo{}, err
	}
	img, info := SizeDeviceImage(composite, deviceCfg)
	return img, info, nil
}

// CompositeDeviceImage produces the device at its source's native size: the
// screenshot warped into its frame, or the flat screenshot as it is on disk.
//
// This is the half that does not depend on the configured width or height, and
// it is nearly all of the cost — loading two 3840x2160 PNGs and compositing
// them runs to about 350ms. Split from the sizing so a caller sweeping through
// heights pays it once instead of once per step.
func CompositeDeviceImage(deviceCfg *config.DeviceImage, screenshotsDir, deviceName string) (image.Image, error) {
	fullPath := DeviceSourcePath(deviceCfg, screenshotsDir, deviceName)

	if deviceCfg.Template == "" {
		// Flat mode: load screenshot directly
		img, err := imaging.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load device image: %w", err)
		}
		return img, nil
	}

	// Template mode: load frame JSON and composite screenshot
	templatePath := screenshotsDir + "/" + deviceCfg.Template
	frame, mask, meta, err := rotato.LoadFrame(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load device template %s: %w", templatePath, err)
	}
	img, err := rotato.RenderWithFrame(frame, mask, meta, fullPath, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to render with device template: %w", err)
	}
	return img, nil
}

// SizeDeviceImage applies the config's sizing and cropping to a composite.
//
// The order is the pipeline's own and is load-bearing: scale the whole thing to
// the target, crop to what is left opaque, then scale the *crop* to the target
// again. That is why `height` ends up sizing the device plus its shadow rather
// than the device, and why crop_padding is measured in scaled pixels.
func SizeDeviceImage(img image.Image, deviceCfg *config.DeviceImage) (image.Image, DeviceImageInfo) {
	if deviceCfg.Width > 0 || deviceCfg.Height > 0 {
		img = rotato.ScaleImage(img, deviceCfg.Width, deviceCfg.Height)
	}

	info := DeviceImageInfo{Stage: img.Bounds().Size(), Crop: img.Bounds()}

	// Auto-crop to content bounds if enabled
	if deviceCfg.AutoCrop {
		threshold := uint8(deviceCfg.CropThreshold)
		if threshold == 0 {
			threshold = 10 // Default: include soft shadow edges
		}
		padding := deviceCfg.CropPadding

		info.Crop = rotato.ContentCropRect(img, threshold, padding)
		img = imaging.Crop(img, info.Crop)
	}

	// Scale after cropping if dimensions specified
	if deviceCfg.AutoCrop && (deviceCfg.Width > 0 || deviceCfg.Height > 0) {
		img = scaleImageToFit(img, deviceCfg.Width, deviceCfg.Height)
	}

	info.Final = img.Bounds().Size()
	return img, info
}

// DrawDeviceImage places a prepared device image on the canvas: horizontally
// centred, plus the config's X offset, at the config's Y.
//
// What gets centred here is the image, and with auto_crop the image is the
// device plus its drop shadow. Rotato renders that shadow off to one side, so
// centring the image does not centre the device and X: 0 is not "centred" —
// see internal/geometry, which models exactly where the device lands.
func DrawDeviceImage(dc *gg.Context, deviceCfg *config.DeviceImage, img image.Image) {
	imgW := img.Bounds().Dx()
	x := (float64(dc.Width())-float64(imgW))/2 + deviceCfg.X
	y := deviceCfg.Y

	dc.DrawImage(img, int(x), int(y))
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
