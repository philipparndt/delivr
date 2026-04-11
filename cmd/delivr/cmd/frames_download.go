package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/philipparndt/delivr/internal/frames"
	"github.com/spf13/cobra"
)

var downloadOutputDir string
var downloadDevice string
var downloadURL string
var downloadList bool

var framesDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Apple device bezels from Apple's CDN",
	Long:  GetHelp("frames-download"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if downloadList {
			fmt.Println("Available devices:")
			fmt.Println()
			for _, slug := range frames.CatalogSlugs() {
				entry := frames.DeviceCatalog[slug]
				fmt.Printf("  %-25s %s\n", slug, entry.Name)
			}
			return nil
		}

		if downloadURL != "" {
			return runDownloadURL(downloadURL, downloadOutputDir, verbose)
		}

		if downloadOutputDir == "" {
			return fmt.Errorf("--output is required")
		}

		if err := frames.CheckHdiutil(); err != nil {
			return err
		}

		if downloadDevice != "" {
			return runDownloadDevice(downloadDevice, downloadOutputDir, verbose)
		}

		return runDownloadAll(downloadOutputDir, verbose)
	},
}

func init() {
	framesDownloadCmd.Flags().StringVar(&downloadOutputDir, "output", "", "Output directory for downloaded bezel PNGs (required)")
	framesDownloadCmd.Flags().StringVar(&downloadDevice, "device", "", "Download only a specific device (e.g., iphone-17)")
	framesDownloadCmd.Flags().StringVar(&downloadURL, "url", "", "Download from a custom DMG URL")
	framesDownloadCmd.Flags().BoolVar(&downloadList, "list", false, "List all available devices in the catalog")
	framesCmd.AddCommand(framesDownloadCmd)
}

func runDownloadDevice(slug, outputDir string, verbose bool) error {
	entry, ok := frames.DeviceCatalog[slug]
	if !ok {
		return fmt.Errorf("unknown device %q — run 'delivr frames download --list' to see available devices", slug)
	}

	if frames.HasExistingPNGs(outputDir, slug) {
		fmt.Printf("Skipping %s — already downloaded\n", entry.Name)
		return nil
	}

	fmt.Printf("Downloading %s...\n", entry.Name)
	files, err := frames.DownloadAndExtract(entry.URL, outputDir, verbose)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", entry.Name, err)
	}
	fmt.Printf("Extracted %d PNG files to %s\n", len(files), outputDir)
	return nil
}

func runDownloadAll(outputDir string, verbose bool) error {
	slugs := frames.CatalogSlugs()
	fmt.Printf("Downloading %d device bezels...\n", len(slugs))

	downloaded := 0
	skipped := 0

	for i, slug := range slugs {
		entry := frames.DeviceCatalog[slug]

		if frames.HasExistingPNGs(outputDir, slug) {
			fmt.Printf("[%d/%d] %s — skipped (already exists)\n", i+1, len(slugs), entry.Name)
			skipped++
			continue
		}

		fmt.Printf("[%d/%d] %s\n", i+1, len(slugs), entry.Name)
		files, err := frames.DownloadAndExtract(entry.URL, outputDir, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v (skipping)\n", err)
			continue
		}
		fmt.Printf("  Extracted %d PNG files\n", len(files))
		downloaded++
	}

	fmt.Printf("\nDone! Downloaded %d, skipped %d. Bezels in %s\n", downloaded, skipped, outputDir)
	return nil
}

func runDownloadURL(url, outputDir string, verbose bool) error {
	if outputDir == "" {
		return fmt.Errorf("--output is required")
	}

	if err := frames.CheckHdiutil(); err != nil {
		return err
	}

	fmt.Printf("Downloading from %s...\n", filepath.Base(url))
	files, err := frames.DownloadAndExtract(url, outputDir, verbose)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d PNG files to %s\n", len(files), outputDir)
	return nil
}
