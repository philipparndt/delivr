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

var rotatoTemplateFile string
var rotatoImagesInput string
var rotatoOutputDir string
var rotatoFramesDir string

var rotatoCmd = &cobra.Command{
	Use:   "rotato",
	Short: "Batch process images through Rotato 3D",
	Long:  GetHelp("rotato"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if rotatoTemplateFile == "" || rotatoImagesInput == "" || rotatoOutputDir == "" {
			return fmt.Errorf("--template, --images, and --output are required")
		}
		return runRotatoBatch(rotatoTemplateFile, rotatoImagesInput, rotatoOutputDir, rotatoFramesDir, verbose)
	},
}

func init() {
	rotatoCmd.Flags().StringVar(&rotatoTemplateFile, "template", "", "Path to .rotato template file (required)")
	rotatoCmd.Flags().StringVar(&rotatoImagesInput, "images", "", "Directory or comma-separated list of image files (required)")
	rotatoCmd.Flags().StringVar(&rotatoOutputDir, "output", "", "Output directory for 3D rendered images (required)")
	rotatoCmd.Flags().StringVar(&rotatoFramesDir, "frames", "", "Directory containing pre-rendered frame PNG+JSON (optional; enables fast path)")
	rootCmd.AddCommand(rotatoCmd)
}

func runRotatoBatch(templateFile, imagesInput, outputDir, framesDir string, verbose bool) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	var frameImage *image.NRGBA
	var frameMask *image.NRGBA
	var frameMeta *rotato.FrameMetadata
	if framesDir != "" {
		jsonPath := rotato.FrameJSONForTemplate(templateFile, framesDir)
		if _, err := os.Stat(jsonPath); err == nil {
			img, mask, meta, loadErr := rotato.LoadFrame(jsonPath)
			if loadErr != nil {
				return fmt.Errorf("failed to load frame %s: %w", jsonPath, loadErr)
			}
			frameImage = img
			frameMask = mask
			frameMeta = meta
			if verbose {
				fmt.Printf("Using pre-rendered frame: %s\n", jsonPath)
				if meta.IsRectangle {
					fmt.Printf("  Mode: rectangle blit\n")
				} else {
					fmt.Printf("  Mode: perspective warp\n")
				}
			}
		} else if verbose {
			fmt.Printf("No frame found at %s, falling back to Rotato UI automation\n", jsonPath)
		}
	}

	type imageFile struct {
		fullPath     string
		relativePath string
	}
	var imageFiles []imageFile
	var baseDir string

	if info, err := os.Stat(imagesInput); err == nil && info.IsDir() {
		baseDir = imagesInput
		err := filepath.Walk(imagesInput, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".png") {
				relPath, _ := filepath.Rel(imagesInput, path)
				imageFiles = append(imageFiles, imageFile{
					fullPath:     path,
					relativePath: relPath,
				})
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		for _, f := range strings.Split(imagesInput, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				imageFiles = append(imageFiles, imageFile{
					fullPath:     f,
					relativePath: filepath.Base(f),
				})
			}
		}
	}

	if len(imageFiles) == 0 {
		return fmt.Errorf("no image files found in %s", imagesInput)
	}

	if frameImage != nil {
		fmt.Printf("Processing %d images using pre-rendered frame (fast path)...\n", len(imageFiles))
	} else {
		fmt.Printf("Processing %d images through Rotato UI automation (slow path)...\n", len(imageFiles))
	}
	if baseDir != "" && verbose {
		fmt.Printf("Base directory: %s\n", baseDir)
	}

	for i, img := range imageFiles {
		outputPath := filepath.Join(outputDir, img.relativePath)

		outputSubDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputSubDir, 0755); err != nil {
			return fmt.Errorf("failed to create output subdirectory: %w", err)
		}

		if verbose {
			fmt.Printf("\n[%d/%d] %s\n", i+1, len(imageFiles), img.relativePath)
		} else {
			fmt.Printf("Processing: %s\n", img.relativePath)
		}

		var rendered image.Image
		var err error
		if frameImage != nil {
			rendered, err = rotato.RenderWithFrame(frameImage, frameMask, frameMeta, img.fullPath, 0, 0)
		} else {
			rendered, err = rotato.RenderWithCLI(templateFile, img.fullPath, 0, 0, verbose)
		}
		if err != nil {
			return fmt.Errorf("failed to process %s: %w", img.relativePath, err)
		}

		if err := imaging.Save(rendered, outputPath); err != nil {
			return fmt.Errorf("failed to save %s: %w", outputPath, err)
		}

		fmt.Printf("Generated: %s\n", outputPath)
	}

	fmt.Printf("\nDone! Processed %d images.\n", len(imageFiles))
	return nil
}
