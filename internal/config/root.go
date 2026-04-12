package config

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RootConfig is the unified delivr.yaml configuration that ties together
// capture, generate, and deliver configs with shared device definitions.
type RootConfig struct {
	Settings Settings                `yaml:"settings"`
	Devices  map[string]UnifiedDevice `yaml:"devices"`
	Capture  *CaptureSection         `yaml:"capture,omitempty"`
	Generate *GenerateSection        `yaml:"generate,omitempty"`
	Deliver  *DeliverSection         `yaml:"deliver,omitempty"`
}

// UnifiedDevice extends the device config with capture-relevant fields.
type UnifiedDevice struct {
	Name             string `yaml:"name"`
	Width            int    `yaml:"width"`
	Height           int    `yaml:"height"`
	Platform         string `yaml:"platform,omitempty"`          // "ios" or "macos"
	ScreenshotPrefix string `yaml:"screenshot_prefix,omitempty"`
	DisplayType      string `yaml:"display_type,omitempty"`
}

// CaptureSection defines capture settings in the root config.
type CaptureSection struct {
	Project         string            `yaml:"project"`
	Scheme          string            `yaml:"scheme"`
	TestTarget      string            `yaml:"test_target"`
	Output          string            `yaml:"output,omitempty"`
	Appearances     []string          `yaml:"appearances,omitempty"`
	StatusBar       StatusBarSection  `yaml:"status_bar,omitempty"`
	Parallel        bool              `yaml:"parallel,omitempty"`
	MacOSWindowSize [2]int            `yaml:"macos_window_size,omitempty"` // [width, height]
}

// StatusBarSection mirrors capture.StatusBarConfig for the root config.
type StatusBarSection struct {
	Time         string `yaml:"time,omitempty"`
	WifiBars     int    `yaml:"wifi_bars,omitempty"`
	CellularBars int    `yaml:"cellular_bars,omitempty"`
	BatteryLevel int    `yaml:"battery_level,omitempty"`
	BatteryState string `yaml:"battery_state,omitempty"`
}

// GenerateSection defines generate settings in the root config.
type GenerateSection struct {
	ScreenshotsDir string                                    `yaml:"screenshots_dir,omitempty"`
	Output         string                                    `yaml:"output,omitempty"`
	Templates      map[string]ScreenTemplate                 `yaml:"templates,omitempty"`
	Screens        []Screen                                  `yaml:"screens"`
	Outputs        []Output                                  `yaml:"outputs"`
	Languages      []string                                  `yaml:"languages,omitempty"`
	Translations   map[string]map[string]ScreenTranslation   `yaml:"translations,omitempty"`
	LanguageFonts  map[string]LanguageFontConfig              `yaml:"language_fonts,omitempty"`
}

// DeliverSection defines deliver settings in the root config.
type DeliverSection struct {
	MetadataDir    string `yaml:"metadata_dir,omitempty"`
	ScreenshotsDir string `yaml:"screenshots_dir,omitempty"`
}

// IsRootConfig checks if the given YAML data represents a root config
// (has capture/generate/deliver keys) vs a standalone config.
func IsRootConfig(path string) (bool, error) {
	data, err := LoadWithIncludes(path)
	if err != nil {
		return false, err
	}

	var probe struct {
		Capture  *yaml.Node `yaml:"capture"`
		Generate *yaml.Node `yaml:"generate"`
		Deliver  *yaml.Node `yaml:"deliver"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false, err
	}

	return probe.Capture != nil || probe.Generate != nil || probe.Deliver != nil, nil
}

// LoadRootConfig loads and parses a root delivr.yaml config.
func LoadRootConfig(path string) (*RootConfig, error) {
	data, err := LoadWithIncludes(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load root config: %w", err)
	}

	var cfg RootConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse root config: %w", err)
	}

	// Resolve paths relative to config file
	absPath, _ := filepath.Abs(path)
	configDir := filepath.Dir(absPath)

	cfg.Settings.FontsDir = resolvePath(configDir, cfg.Settings.FontsDir)
	if cfg.Settings.ScreenshotsDir != "" {
		cfg.Settings.ScreenshotsDir = resolvePath(configDir, cfg.Settings.ScreenshotsDir)
	}

	if cfg.Capture != nil {
		if cfg.Capture.Project != "" {
			cfg.Capture.Project = resolvePath(configDir, cfg.Capture.Project)
		}
		if cfg.Capture.Output != "" {
			cfg.Capture.Output = resolvePath(configDir, cfg.Capture.Output)
		}
	}

	if cfg.Generate != nil {
		if cfg.Generate.ScreenshotsDir != "" {
			cfg.Generate.ScreenshotsDir = resolvePath(configDir, cfg.Generate.ScreenshotsDir)
		}
		if cfg.Generate.Output != "" {
			cfg.Generate.Output = resolvePath(configDir, cfg.Generate.Output)
		}
	}

	if cfg.Deliver != nil {
		if cfg.Deliver.MetadataDir != "" {
			cfg.Deliver.MetadataDir = resolvePath(configDir, cfg.Deliver.MetadataDir)
		}
		if cfg.Deliver.ScreenshotsDir != "" {
			cfg.Deliver.ScreenshotsDir = resolvePath(configDir, cfg.Deliver.ScreenshotsDir)
		}
	}

	return &cfg, nil
}

// ToGenerateConfig converts a RootConfig into a Config suitable for the generate command.
func (r *RootConfig) ToGenerateConfig() (*Config, error) {
	cfg := &Config{
		Settings: r.Settings,
		Devices:  make(map[string]Device),
	}

	for key, ud := range r.Devices {
		cfg.Devices[key] = Device{
			Name:             ud.Name,
			Width:            ud.Width,
			Height:           ud.Height,
			ScreenshotPrefix: ud.ScreenshotPrefix,
			DisplayType:      ud.DisplayType,
		}
	}

	if r.Generate != nil {
		cfg.Templates = r.Generate.Templates
		cfg.Screens = r.Generate.Screens
		cfg.Outputs = r.Generate.Outputs
		cfg.Languages = r.Generate.Languages
		cfg.Translations = r.Generate.Translations
		cfg.LanguageFonts = r.Generate.LanguageFonts
		if r.Generate.ScreenshotsDir != "" {
			cfg.Settings.ScreenshotsDir = r.Generate.ScreenshotsDir
		}
	}

	// Apply templates to screens (merge template properties into screen configs)
	if err := applyTemplates(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply templates: %w", err)
	}

	return cfg, nil
}
