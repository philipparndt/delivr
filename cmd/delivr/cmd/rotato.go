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

var rotatoInputDir string
var rotatoOutputDir string
var rotatoDim string

var rotatoCmd = &cobra.Command{
	Use:   "rotato",
	Short: "Generate device templates from .rotato files",
	Long:  GetHelp("rotato"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if rotatoInputDir == "" || rotatoOutputDir == "" {
			return fmt.Errorf("--input and --output are required")
		}

		w, h, err := parseDim(rotatoDim)
		if err != nil {
			return fmt.Errorf("invalid --dim: %w", err)
		}

		return runRotatoGenerate(rotatoInputDir, rotatoOutputDir, w, h, verbose)
	},
}

func init() {
	rotatoCmd.Flags().StringVar(&rotatoInputDir, "input", "", "Directory containing .rotato files (required)")
	rotatoCmd.Flags().StringVar(&rotatoOutputDir, "output", "", "Output directory for device template sets (required)")
	rotatoCmd.Flags().StringVar(&rotatoDim, "dim", "1320x2868", "Placeholder dimensions as WxH")
	rootCmd.AddCommand(rotatoCmd)
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

func runRotatoGenerate(inputDir, outputDir string, w, h int, verbose bool) error {
	// Find all .rotato files in input directory
	rotatoFiles, err := rotato.FindRotatoFiles(inputDir)
	if err != nil {
		return fmt.Errorf("failed to scan input directory: %w", err)
	}

	if len(rotatoFiles) == 0 {
		return fmt.Errorf("no .rotato files found in %s", inputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Found %d .rotato files. Generating device templates...\n", len(rotatoFiles))

	for i, rotatoFile := range rotatoFiles {
		stem := strings.TrimSuffix(filepath.Base(rotatoFile), filepath.Ext(rotatoFile))
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(rotatoFiles), filepath.Base(rotatoFile))

		// Check for cached raw render
		rawPath := filepath.Join(outputDir, stem+".raw.png")
		var rendered image.Image

		if cached, err := loadRawIfExists(rawPath); err == nil && cached != nil {
			fmt.Printf("  Using cached raw render: %s\n", rawPath)
			rendered = cached
		} else {
			// Generate magenta placeholder and render through Rotato
			tmpDir, err := os.MkdirTemp("", "rotato-placeholder")
			if err != nil {
				return fmt.Errorf("failed to create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			placeholder := filepath.Join(tmpDir, "placeholder.png")
			if verbose {
				fmt.Printf("  Generating %dx%d magenta placeholder\n", w, h)
			}
			if err := rotato.GeneratePlaceholder(w, h, placeholder); err != nil {
				return fmt.Errorf("failed to create placeholder: %w", err)
			}

			fmt.Printf("  Rendering through Rotato (UI automation, this takes ~20s)...\n")
			rendered, err = rotato.RenderWithCLI(rotatoFile, placeholder, 0, 0, verbose)
			if err != nil {
				return fmt.Errorf("rotato render failed for %s: %w", filepath.Base(rotatoFile), err)
			}

			// Cache the raw render
			if err := saveRawPNG(rawPath, rendered); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: could not cache raw render: %v\n", err)
			} else if verbose {
				fmt.Printf("  Saved raw render: %s\n", rawPath)
			}
		}

		// Detect frame and save template set
		cleaned, mask, meta, err := rotato.DetectFrame(rendered, rotatoFile, w, h)
		if err != nil {
			return fmt.Errorf("frame analysis failed for %s: %w", filepath.Base(rotatoFile), err)
		}

		pngPath, maskPath, jsonPath, err := rotato.SaveFrame(cleaned, mask, meta, outputDir, rotatoFile)
		if err != nil {
			return fmt.Errorf("failed to save template for %s: %w", filepath.Base(rotatoFile), err)
		}

		// Save debug overlay
		debugPath, derr := rotato.SaveDebugFrame(cleaned, meta, outputDir, rotatoFile)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not save debug overlay: %v\n", derr)
		}

		fmt.Printf("  Template saved:\n    PNG:  %s\n    Mask: %s\n    JSON: %s\n", pngPath, maskPath, jsonPath)
		if derr == nil {
			fmt.Printf("    Debug: %s\n", debugPath)
		}
		if meta.IsRectangle {
			fmt.Printf("  Screen: rectangle %dx%d at (%d, %d)\n",
				meta.RectangleRect[2], meta.RectangleRect[3],
				meta.RectangleRect[0], meta.RectangleRect[1])
		} else {
			fmt.Printf("  Screen: perspective quad TL=%v TR=%v BR=%v BL=%v\n",
				meta.Corners[0], meta.Corners[1], meta.Corners[2], meta.Corners[3])
		}
	}

	fmt.Printf("\nDone! Generated %d device template sets in %s\n", len(rotatoFiles), outputDir)
	return nil
}
