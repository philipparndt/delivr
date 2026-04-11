package cmd

import (
	"fmt"

	"github.com/philipparndt/delivr/internal/capture"
	"github.com/philipparndt/delivr/internal/config"
	"github.com/spf13/cobra"
)

func captureConfigFromRoot(root *config.RootConfig) *capture.Config {
	cfg := &capture.Config{
		Project:         root.Capture.Project,
		Scheme:          root.Capture.Scheme,
		TestTarget:      root.Capture.TestTarget,
		Output:          root.Capture.Output,
		Appearances:     root.Capture.Appearances,
		Parallel:        root.Capture.Parallel,
		MacOSWindowSize: root.Capture.MacOSWindowSize,
		StatusBar: capture.StatusBarConfig{
			Time:         root.Capture.StatusBar.Time,
			WifiBars:     root.Capture.StatusBar.WifiBars,
			CellularBars: root.Capture.StatusBar.CellularBars,
			BatteryLevel: root.Capture.StatusBar.BatteryLevel,
			BatteryState: root.Capture.StatusBar.BatteryState,
		},
	}

	for _, ud := range root.Devices {
		if ud.Platform != "" {
			cfg.Devices = append(cfg.Devices, capture.DeviceConfig{
				Name:     ud.Name,
				Platform: ud.Platform,
			})
		}
	}

	// Defaults
	if cfg.StatusBar.Time == "" { cfg.StatusBar.Time = "9:41" }
	if cfg.StatusBar.WifiBars == 0 { cfg.StatusBar.WifiBars = 3 }
	if cfg.StatusBar.CellularBars == 0 { cfg.StatusBar.CellularBars = 4 }
	if cfg.StatusBar.BatteryLevel == 0 { cfg.StatusBar.BatteryLevel = 100 }
	if cfg.StatusBar.BatteryState == "" { cfg.StatusBar.BatteryState = "charged" }
	if len(cfg.Appearances) == 0 { cfg.Appearances = []string{"light"} }
	if cfg.MacOSWindowSize == [2]int{0, 0} { cfg.MacOSWindowSize = [2]int{1200, 720} }

	return cfg
}

var captureConfigFile string

var captureCmd = &cobra.Command{
	Use:          "capture",
	Short:        "Capture screenshots from iOS simulators and macOS",
	Long:         GetHelp("capture"),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if captureConfigFile == "" {
			return cmd.Help()
		}

		// Auto-detect root config vs standalone capture config
		isRoot, _ := config.IsRootConfig(captureConfigFile)
		if isRoot {
			rootCfg, err := config.LoadRootConfig(captureConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if rootCfg.Capture == nil {
				return fmt.Errorf("root config has no 'capture' section")
			}
			capCfg := captureConfigFromRoot(rootCfg)
			return capture.RunCapture(capCfg, verbose)
		}

		cfg, err := capture.LoadConfig(captureConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		return capture.RunCapture(cfg, verbose)
	},
}

func init() {
	captureCmd.Flags().StringVar(&captureConfigFile, "config", "", "Path to capture configuration YAML file")
	rootCmd.AddCommand(captureCmd)
}
