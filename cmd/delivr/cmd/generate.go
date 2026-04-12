package cmd

import (
	"fmt"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/render"
	"github.com/spf13/cobra"
)

var configPath string
var outputDir string

var generateCmd = &cobra.Command{
	Use:          "generate",
	Short:        "Generate App Store screenshots from config",
	Long:         GetHelp("generate"),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := configPath
		if cfg == "" {
			// Check if root command's config flag was used
			if rootFlag := rootCmd.Flags().Lookup("config"); rootFlag != nil && rootFlag.Changed {
				cfg = rootFlag.Value.String()
			}
		}
		if cfg == "" {
			return fmt.Errorf("--config is required")
		}

		out := outputDir
		if out == "" {
			out = "./output"
		}

		return runGenerate(cfg, out, verbose)
	},
}

func init() {
	generateCmd.Flags().StringVar(&configPath, "config", "", "Path to configuration YAML file")
	generateCmd.Flags().StringVar(&outputDir, "output", "./output", "Output directory for generated images")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(configPath, outputDir string, verbose bool) error {
	if verbose {
		fmt.Printf("Loading config from %s\n", configPath)
	}

	// Auto-detect root config vs standalone
	var cfg *config.Config
	isRoot, _ := config.IsRootConfig(configPath)
	if isRoot {
		rootCfg, err := config.LoadRootConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg, err = rootCfg.ToGenerateConfig()
		if err != nil {
			return fmt.Errorf("failed to prepare generate config: %w", err)
		}
		if rootCfg.Generate != nil && rootCfg.Generate.Output != "" && outputDir == "./output" {
			outputDir = rootCfg.Generate.Output
		}
	} else {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	if verbose {
		fmt.Printf("Config loaded: %d devices, %d screens, %d outputs\n",
			len(cfg.Devices), len(cfg.Screens), len(cfg.Outputs))
		fmt.Printf("Fonts directory: %s\n", cfg.Settings.FontsDir)
		fmt.Printf("Screenshots directory: %s\n", cfg.Settings.ScreenshotsDir)
		fmt.Printf("Output directory: %s\n", outputDir)
	}

	renderer := render.NewRenderer(cfg, outputDir, verbose)
	defer renderer.Close()

	if err := renderer.RenderAll(); err != nil {
		return err
	}

	if verbose {
		fmt.Println("Done!")
	}

	return nil
}
