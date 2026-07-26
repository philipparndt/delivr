package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/video"
	"github.com/spf13/cobra"
)

var (
	videoConfigPath string
	videoOutputDir  string
	videoOnlyDevice string
	videoSkipRecord bool
)

var videoCmd = &cobra.Command{
	Use:          "video",
	Short:        "Record and encode App Store preview videos",
	Long:         GetHelp("video"),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := videoConfigPath
		if cfgPath == "" {
			if rootFlag := rootCmd.Flags().Lookup("config"); rootFlag != nil && rootFlag.Changed {
				cfgPath = rootFlag.Value.String()
			}
		}
		if cfgPath == "" {
			return fmt.Errorf("--config is required")
		}
		return runVideo(cfgPath, videoOutputDir, videoOnlyDevice, videoSkipRecord, verbose)
	},
}

func init() {
	videoCmd.Flags().StringVar(&videoConfigPath, "config", "", "Path to configuration YAML file")
	videoCmd.Flags().StringVar(&videoOutputDir, "output", "", "Output directory (defaults to video.output)")
	videoCmd.Flags().StringVar(&videoOnlyDevice, "device", "", "Only this device key")
	videoCmd.Flags().BoolVar(&videoSkipRecord, "skip-record", false,
		"Re-encode the existing raw capture instead of filming again")
	rootCmd.AddCommand(videoCmd)
}

func runVideo(cfgPath, outputDir, only string, skipRecord, verbose bool) error {
	cfg, err := config.LoadRootConfig(cfgPath)
	if err != nil {
		return err
	}
	if cfg.Video == nil {
		return fmt.Errorf("no `video:` section in %s", cfgPath)
	}
	if len(cfg.Video.Devices) == 0 {
		return fmt.Errorf("`video.devices` is empty — nothing to record")
	}

	out := outputDir
	if out == "" {
		out = cfg.Video.Output
	}
	if out == "" {
		out = "./output/previews"
	}

	logf := func(format string, args ...any) {
		fmt.Printf("    "+format+"\n", args...)
	}
	recorder := &video.Recorder{Verbose: verbose, Log: logf}
	processor := &video.Processor{Verbose: verbose, Log: logf}

	// Deterministic order, so a run reads the same way twice.
	keys := make([]string, 0, len(cfg.Video.Devices))
	for k := range cfg.Video.Devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if only != "" && key != only {
			continue
		}
		dev := cfg.Video.Devices[key]

		// Fall back to the shared device definition, so sizes and the ASC type
		// are declared once in devices.yaml.
		unified, ok := cfg.Devices[key]
		if !ok {
			return fmt.Errorf("video device %q has no entry in `devices`", key)
		}
		if dev.Width == 0 {
			dev.Width = unified.Width
		}
		if dev.Height == 0 {
			dev.Height = unified.Height
		}
		if dev.Simulator == "" {
			dev.Simulator = unified.Name
		}

		fmt.Printf("==> %s (%dx%d)\n", key, dev.Width, dev.Height)

		rawDir := filepath.Join(out, ".raw", key)
		raw := filepath.Join(rawDir, "raw.mov")

		if skipRecord {
			if _, err := os.Stat(raw); err != nil {
				return fmt.Errorf("--skip-record but no capture at %s", raw)
			}
			logf("reusing %s", raw)
		} else {
			if cfg.Video.Record == nil {
				return fmt.Errorf("`video.record` is required unless --skip-record is passed")
			}
			rec := *cfg.Video.Record
			// Per-device bundle wins: an iOS+tvOS project builds a separate
			// .app per platform and cannot share one path.
			if dev.App != "" {
				rec.App = dev.App
			}
			raw, err = recorder.Record(dev, rec,
				unified.Name, cfg.Settings.BundleID, rawDir)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}

		// Per-device soundtrack wins outright, for the same reason as the
		// bundle above: what each recording shows can differ per platform, and
		// so can the audio that belongs under it.
		final := filepath.Join(out, fmt.Sprintf("%s.mp4", key))
		audio := video.AudioFor(dev, cfg.Video.Audio)
		if err := processor.Process(raw, dev, audio, final); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}

	fmt.Printf("\nPreviews in %s\n", out)
	return nil
}
