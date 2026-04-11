package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "delivr",
	Short: "App Store screenshot generator and delivery tool",
	Long:  GetHelp("root"),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to generate if --config is provided
		if configPath != "" {
			return runGenerate(configPath, outputDir, verbose)
		}
		return cmd.Help()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	// Add --config and --output to root so `delivr --config ...` works as shorthand for generate
	rootCmd.Flags().StringVar(&configPath, "config", "", "Path to configuration YAML file")
	rootCmd.Flags().StringVar(&outputDir, "output", "./output", "Output directory for generated images")
}
