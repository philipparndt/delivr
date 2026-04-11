package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/philipparndt/delivr/internal/asc"
	"github.com/philipparndt/delivr/internal/config"
	"github.com/spf13/cobra"
)

var deliverConfigFile string
var deliverScreenshotsDir string
var deliverMetadataDir string
var deliverKeyID string
var deliverIssuerID string
var deliverKeyFile string
var deliverKeyPEM string
var deliverSkipMeta bool
var deliverSkipScreens bool

// localeFileSuffix maps ASC locale codes to file suffixes used in this project.
var localeFileSuffix = map[string]string{
	"en-US":   "",
	"de-DE":   "de",
	"es-ES":   "es",
	"fr-FR":   "fr",
	"ja":      "ja",
	"pt-BR":   "pt-BR",
	"ko":      "ko",
	"zh-Hans": "zh-Hans",
}

var deliverCmd = &cobra.Command{
	Use:   "deliver",
	Short: "Upload metadata and screenshots to App Store Connect",
	Long:  GetHelp("deliver"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deliverConfigFile == "" {
			return fmt.Errorf("--config is required")
		}

		keySource := deliverKeyPEM
		if keySource == "" {
			keySource = deliverKeyFile
		}
		if deliverKeyID == "" || deliverIssuerID == "" || keySource == "" {
			return fmt.Errorf("API credentials required (--key-id, --issuer-id, and --key-file or --key-pem)")
		}

		// Auto-detect root config vs standalone
		var cfg *config.Config
		isRoot, _ := config.IsRootConfig(deliverConfigFile)
		if isRoot {
			rootCfg, err := config.LoadRootConfig(deliverConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			cfg = rootCfg.ToGenerateConfig()
			if rootCfg.Deliver != nil && rootCfg.Deliver.MetadataDir != "" && deliverMetadataDir == "" {
				deliverMetadataDir = rootCfg.Deliver.MetadataDir
			}
			if rootCfg.Deliver != nil && rootCfg.Deliver.ScreenshotsDir != "" && deliverScreenshotsDir == "" {
				deliverScreenshotsDir = rootCfg.Deliver.ScreenshotsDir
			}
		} else {
			var err error
			cfg, err = config.Load(deliverConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		if cfg.Settings.BundleID == "" {
			return fmt.Errorf("settings.bundle_id not set in config")
		}

		metaDir := deliverMetadataDir
		if metaDir == "" {
			metaDir = filepath.Dir(deliverConfigFile)
		}

		deliverCfg := asc.DeliverConfig{
			BundleID: cfg.Settings.BundleID,
		}

		if !deliverSkipMeta {
			deliverCfg.Metadata = make(map[string]*asc.LocaleMetadata)
			for _, locale := range cfg.Languages {
				meta, err := loadLocaleMetadata(metaDir, locale)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not load metadata for %s: %v\n", locale, err)
					continue
				}
				deliverCfg.Metadata[locale] = meta
			}
		}

		if !deliverSkipScreens {
			screenDir := deliverScreenshotsDir
			if screenDir == "" {
				screenDir = filepath.Join(filepath.Dir(deliverConfigFile), "output", "appstore")
			}
			deliverCfg.Screenshots = gatherScreenshots(cfg, screenDir)
		}

		client, err := asc.NewClient(deliverKeyID, deliverIssuerID, keySource)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		client.Verbose = verbose

		fmt.Println("Delivering to App Store Connect...")
		return client.Deliver(deliverCfg)
	},
}

func init() {
	deliverCmd.PersistentFlags().StringVar(&deliverConfigFile, "config", "", "Path to appstore.yaml config file")
	deliverCmd.PersistentFlags().StringVar(&deliverKeyID, "key-id", os.Getenv("ASC_KEY_ID"), "App Store Connect API Key ID (or ASC_KEY_ID env)")
	deliverCmd.PersistentFlags().StringVar(&deliverIssuerID, "issuer-id", os.Getenv("ASC_ISSUER_ID"), "App Store Connect Issuer ID (or ASC_ISSUER_ID env)")
	deliverCmd.PersistentFlags().StringVar(&deliverKeyFile, "key-file", os.Getenv("ASC_KEY_FILE"), "Path to .p8 private key file (or ASC_KEY_FILE env)")
	deliverCmd.PersistentFlags().StringVar(&deliverKeyPEM, "key-pem", os.Getenv("ASC_KEY_PEM"), "Inline PEM private key content (or ASC_KEY_PEM env)")
	deliverCmd.Flags().StringVar(&deliverScreenshotsDir, "screenshots", "", "Path to generated screenshots (output/appstore)")
	deliverCmd.Flags().StringVar(&deliverMetadataDir, "metadata", "", "Path to metadata files directory (default: config file directory)")
	deliverCmd.Flags().BoolVar(&deliverSkipMeta, "skip-metadata", false, "Skip metadata upload")
	deliverCmd.Flags().BoolVar(&deliverSkipScreens, "skip-screenshots", false, "Skip screenshot upload")
	rootCmd.AddCommand(deliverCmd)
}

func loadLocaleMetadata(baseDir, locale string) (*asc.LocaleMetadata, error) {
	suffix, ok := localeFileSuffix[locale]
	if !ok {
		return nil, fmt.Errorf("unknown locale %s", locale)
	}

	meta := &asc.LocaleMetadata{}

	descFile := "appstore-text.md"
	if suffix != "" {
		descFile = fmt.Sprintf("appstore-text-%s.md", suffix)
	}
	if desc, err := readTrimmedFile(filepath.Join(baseDir, descFile)); err == nil {
		meta.Description = desc
	}

	promoFile := "appstore-promo.md"
	if suffix != "" {
		promoFile = fmt.Sprintf("appstore-promo-%s.md", suffix)
	}
	if promo, err := readTrimmedFile(filepath.Join(baseDir, promoFile)); err == nil {
		meta.PromotionalText = promo
	}

	newsFile := "news.md"
	if suffix != "" {
		localizedNews := fmt.Sprintf("news-%s.md", suffix)
		if _, err := os.Stat(filepath.Join(baseDir, localizedNews)); err == nil {
			newsFile = localizedNews
		}
	}
	if news, err := readTrimmedFile(filepath.Join(baseDir, newsFile)); err == nil {
		meta.WhatsNew = news
	}

	if meta.Description == "" && meta.PromotionalText == "" && meta.WhatsNew == "" {
		return nil, fmt.Errorf("no metadata found")
	}

	return meta, nil
}

func readTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func gatherScreenshots(cfg *config.Config, screenshotsDir string) map[string]map[string][]string {
	result := make(map[string]map[string][]string)

	for _, output := range cfg.Outputs {
		device, ok := cfg.Devices[output.Device]
		if !ok || device.DisplayType == "" {
			continue
		}

		for _, locale := range cfg.Languages {
			langDir := filepath.Join(screenshotsDir, locale, output.Prefix)

			pattern := filepath.Join(langDir, "*.png")
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) == 0 {
				continue
			}

			if result[locale] == nil {
				result[locale] = make(map[string][]string)
			}
			result[locale][device.DisplayType] = append(result[locale][device.DisplayType], matches...)
		}
	}

	return result
}
