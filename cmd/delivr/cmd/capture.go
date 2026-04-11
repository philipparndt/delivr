package cmd

import (
	"fmt"

	"github.com/philipparndt/delivr/internal/capture"
	"github.com/spf13/cobra"
)

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
