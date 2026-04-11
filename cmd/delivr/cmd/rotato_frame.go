package cmd

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/philipparndt/delivr/internal/rotato"
	"github.com/spf13/cobra"
)

var frameTemplateFile string
var frameOutputDir string
var frameDim string

var rotatoFrameCmd = &cobra.Command{
	Use:   "frame",
	Short: "Pre-render a Rotato template to a reusable frame",
	Long:  GetHelp("rotato-frame"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if frameTemplateFile == "" || frameOutputDir == "" {
			return fmt.Errorf("--template and --output are required")
		}

		w, h, err := parseDim(frameDim)
		if err != nil {
			return fmt.Errorf("invalid --dim: %w", err)
		}

		return runRotatoFrame(frameTemplateFile, frameOutputDir, w, h, verbose)
	},
}

func init() {
	rotatoFrameCmd.Flags().StringVar(&frameTemplateFile, "template", "", "Path to .rotato template file (required)")
	rotatoFrameCmd.Flags().StringVar(&frameOutputDir, "output", "", "Directory to write the frame PNG + JSON (required)")
	rotatoFrameCmd.Flags().StringVar(&frameDim, "dim", "1320x2868", "Placeholder dimensions as WxH")
	rotatoCmd.AddCommand(rotatoFrameCmd)
}

func parseDim(s string) (int, int, error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected format WxH, got %q", s)
	}
	var w, h int
	if _, err := fmt.Sscanf(parts[0], "%d", &w); err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("invalid width %q", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &h); err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("invalid height %q", parts[1])
	}
	return w, h, nil
}

func loadRawIfExists(path string) (image.Image, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return imaging.Open(path)
}

func saveRawPNG(path string, img image.Image) error {
	return imaging.Save(img, path)
}

func runRotatoFrame(templateFile, outputDir string, w, h int, verbose bool) error {
	stem := strings.TrimSuffix(filepath.Base(templateFile), filepath.Ext(templateFile))
	rawPath := filepath.Join(outputDir, stem+".raw.png")

	var rendered image.Image
	if cached, err := loadRawIfExists(rawPath); err == nil && cached != nil {
		fmt.Printf("Using cached raw render: %s\n", rawPath)
		rendered = cached
	} else {
		tmpDir, err := os.MkdirTemp("", "rotato-frame-placeholder")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		placeholder := filepath.Join(tmpDir, "placeholder.png")
		if verbose {
			fmt.Printf("Generating %dx%d magenta placeholder at %s\n", w, h, placeholder)
		}
		if err := rotato.GeneratePlaceholder(w, h, placeholder); err != nil {
			return fmt.Errorf("failed to create placeholder: %w", err)
		}

		fmt.Printf("Rendering %s through Rotato (UI automation, this takes ~20s)...\n", filepath.Base(templateFile))
		rendered, err = rotato.RenderWithCLI(templateFile, placeholder, 0, 0, verbose)
		if err != nil {
			return fmt.Errorf("rotato render failed: %w", err)
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := saveRawPNG(rawPath, rendered); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not cache raw render at %s: %v\n", rawPath, err)
		} else if verbose {
			fmt.Printf("Saved raw render: %s\n", rawPath)
		}
	}

	cleaned, mask, meta, err := rotato.DetectFrame(rendered, templateFile, w, h)
	if err != nil {
		return fmt.Errorf("frame analysis failed: %w", err)
	}

	pngPath, maskPath, jsonPath, err := rotato.SaveFrame(cleaned, mask, meta, outputDir, templateFile)
	if err != nil {
		return fmt.Errorf("failed to save frame: %w", err)
	}

	debugPath, derr := rotato.SaveDebugFrame(cleaned, meta, outputDir, templateFile)
	if derr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save debug overlay: %v\n", derr)
	}

	fmt.Printf("Frame saved:\n  PNG:   %s\n  MASK:  %s\n  JSON:  %s\n", pngPath, maskPath, jsonPath)
	if derr == nil {
		fmt.Printf("  Debug: %s\n", debugPath)
	}
	if meta.IsRectangle {
		fmt.Printf("  Screen region: rectangle %dx%d at (%d, %d)\n",
			meta.RectangleRect[2], meta.RectangleRect[3],
			meta.RectangleRect[0], meta.RectangleRect[1])
	} else {
		fmt.Printf("  Screen region: perspective quad TL=%v TR=%v BR=%v BL=%v\n",
			meta.Corners[0], meta.Corners[1], meta.Corners[2], meta.Corners[3])
	}
	return nil
}
