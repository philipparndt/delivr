package cmd

import (
	"fmt"
	"os"

	"github.com/philipparndt/delivr/internal/asc"
	"github.com/philipparndt/delivr/internal/config"
	"github.com/spf13/cobra"
)

var deliverListCmd = &cobra.Command{
	Use:   "list-display-types",
	Short: "List available display types for the app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if deliverConfigFile == "" {
			return fmt.Errorf("--config is required")
		}

		cfg, err := config.Load(deliverConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		keySource := deliverKeyPEM
		if keySource == "" {
			keySource = deliverKeyFile
		}

		client, err := asc.NewClient(deliverKeyID, deliverIssuerID, keySource)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		client.Verbose = true

		if err := client.ListDisplayTypes(cfg.Settings.BundleID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	deliverCmd.AddCommand(deliverListCmd)
}
