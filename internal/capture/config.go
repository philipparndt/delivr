package capture

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config defines the capture configuration for screenshot generation.
type Config struct {
	Project        string          `yaml:"project"`
	Scheme         string          `yaml:"scheme"`
	TestTarget     string          `yaml:"test_target"`
	Output         string          `yaml:"output"`
	Devices        []DeviceConfig  `yaml:"devices"`
	Appearances    []string        `yaml:"appearances"`
	StatusBar      StatusBarConfig `yaml:"status_bar"`
	Parallel       bool            `yaml:"parallel"`
	MacOSWindowSize [2]int         `yaml:"macos_window_size"` // [width, height], default 1200x720
}

// DeviceConfig defines a device to capture screenshots on.
type DeviceConfig struct {
	Name     string `yaml:"name"`
	Platform string `yaml:"platform"` // "ios" or "macos"
}

// StatusBarConfig defines simulator status bar overrides.
type StatusBarConfig struct {
	Time         string `yaml:"time"`
	WifiBars     int    `yaml:"wifi_bars"`
	CellularBars int    `yaml:"cellular_bars"`
	BatteryLevel int    `yaml:"battery_level"`
	BatteryState string `yaml:"battery_state"`
}

// LoadConfig reads and parses a capture configuration YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read capture config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse capture config: %w", err)
	}

	// Resolve project path relative to config file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}
	configDir := filepath.Dir(absPath)

	if !filepath.IsAbs(cfg.Project) {
		cfg.Project = filepath.Join(configDir, cfg.Project)
	}
	if !filepath.IsAbs(cfg.Output) {
		cfg.Output = filepath.Join(configDir, cfg.Output)
	}

	// Defaults
	if cfg.StatusBar.Time == "" {
		cfg.StatusBar.Time = "9:41"
	}
	if cfg.StatusBar.WifiBars == 0 {
		cfg.StatusBar.WifiBars = 3
	}
	if cfg.StatusBar.CellularBars == 0 {
		cfg.StatusBar.CellularBars = 4
	}
	if cfg.StatusBar.BatteryLevel == 0 {
		cfg.StatusBar.BatteryLevel = 100
	}
	if cfg.StatusBar.BatteryState == "" {
		cfg.StatusBar.BatteryState = "charged"
	}
	if len(cfg.Appearances) == 0 {
		cfg.Appearances = []string{"light"}
	}
	if cfg.MacOSWindowSize == [2]int{0, 0} {
		cfg.MacOSWindowSize = [2]int{1200, 720}
	}

	return &cfg, nil
}
