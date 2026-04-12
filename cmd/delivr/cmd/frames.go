package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/philipparndt/delivr/internal/frames"
	"github.com/spf13/cobra"
)

var framesInputDir string
var framesOutputDir string

var framesCmd = &cobra.Command{
	Use:   "frames",
	Short: "Generate device templates from bezel PNGs",
	Long:         GetHelp("frames"),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if framesInputDir == "" || framesOutputDir == "" {
			return cmd.Help()
		}
		return runFramesImport(framesInputDir, framesOutputDir, verbose)
	},
}

func init() {
	framesCmd.Flags().StringVar(&framesInputDir, "input", "", "Directory containing device bezel PNGs (required)")
	framesCmd.Flags().StringVar(&framesOutputDir, "output", "", "Output directory for device template sets (required)")
	rootCmd.AddCommand(framesCmd)
}

func runFramesImport(inputDir, outputDir string, verbose bool) error {
	// Find all PNG files in input directory
	var pngFiles []string
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".png") {
			pngFiles = append(pngFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan input directory: %w", err)
	}

	if len(pngFiles) == 0 {
		return fmt.Errorf("no PNG files found in %s", inputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	total := len(pngFiles)
	workers := runtime.NumCPU()
	fmt.Printf("Generating device templates from %d PNGs (%d workers)\n", total, workers)

	// Progress bar
	bar := progress.New(
		progress.WithGradient("#1A6B5A", "#0F8B6E"),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	var done atomic.Int64
	var skipped atomic.Int64
	var mu sync.Mutex

	printProgress := func(n int64) {
		pct := float64(n) / float64(total)
		label := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			fmt.Sprintf(" %d/%d", n, total),
		)
		mu.Lock()
		fmt.Printf("\r  %s%s", bar.ViewAs(pct), label)
		if n == int64(total) {
			fmt.Println()
		}
		mu.Unlock()
	}

	// Process in parallel
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for _, pngFile := range pngFiles {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			_, _, _, err := frames.GenerateTemplate(path, outputDir, false)
			if err != nil {
				skipped.Add(1)
				if verbose {
					mu.Lock()
					fmt.Fprintf(os.Stderr, "\n  Skipped %s: %v\n", filepath.Base(path), err)
					mu.Unlock()
				}
			}

			n := done.Add(1)
			printProgress(n)
		}(pngFile)
	}

	wg.Wait()

	s := skipped.Load()
	generated := int64(total) - s
	fmt.Printf("Done! Generated %d templates, skipped %d. Output: %s\n", generated, s, outputDir)
	return nil
}
