package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads and parses the YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Get absolute path to config file for resolving relative paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	configDir := filepath.Dir(absPath)
	cfg.Settings.FontsDir = resolvePath(configDir, cfg.Settings.FontsDir)
	cfg.Settings.ScreenshotsDir = resolvePath(configDir, cfg.Settings.ScreenshotsDir)

	// Apply templates to screens
	if err := applyTemplates(&cfg); err != nil {
		return nil, fmt.Errorf("failed to apply templates: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyTemplates merges template properties into screens
func applyTemplates(cfg *Config) error {
	for i := range cfg.Screens {
		screen := &cfg.Screens[i]
		if screen.Template == "" {
			continue
		}

		tmpl, ok := cfg.Templates[screen.Template]
		if !ok {
			return fmt.Errorf("screen %q references unknown template: %s", screen.ID, screen.Template)
		}

		// Merge template into screen (screen values override template)
		screen.Title = mergeTextConfig(tmpl.Title, screen.Title)
		screen.Subtitle = mergeTextConfig(tmpl.Subtitle, screen.Subtitle)
		screen.Background = mergeBackground(tmpl.Background, screen.Background)
		screen.Device = mergeDeviceImage(tmpl.Device, screen.Device)
		screen.Devices = mergeDevicesArray(tmpl.Devices, screen.Devices, tmpl.Device, screen.Device)
	}
	return nil
}

// mergeTextConfig merges two TextConfig, with override taking precedence
func mergeTextConfig(base, override *TextConfig) *TextConfig {
	if base == nil {
		return override
	}
	if override == nil {
		// Copy base to avoid modifying template
		copy := *base
		return &copy
	}

	result := *base
	if override.Text != "" {
		result.Text = override.Text
	}
	if override.Font != "" {
		result.Font = override.Font
	}
	if override.Size != 0 {
		result.Size = override.Size
	}
	if override.Color != "" {
		result.Color = override.Color
	}
	if override.Align != "" {
		result.Align = override.Align
	}
	if override.Y != 0 {
		result.Y = override.Y
	}
	if override.X != 0 {
		result.X = override.X
	}
	return &result
}

// mergeBackground merges two Background, with override taking precedence
func mergeBackground(base, override *Background) *Background {
	if base == nil {
		return override
	}
	if override == nil {
		copy := *base
		return &copy
	}

	result := *base
	if override.Type != "" {
		result.Type = override.Type
	}
	if override.Color != "" {
		result.Color = override.Color
	}
	if override.Angle != 0 {
		result.Angle = override.Angle
	}
	if len(override.Stops) > 0 {
		result.Stops = override.Stops
	}
	return &result
}

// mergeDeviceImage merges two DeviceImage, with override taking precedence
func mergeDeviceImage(base, override *DeviceImage) *DeviceImage {
	if base == nil {
		return override
	}
	if override == nil {
		copy := *base
		return &copy
	}

	result := *base
	if override.Source != "" {
		result.Source = override.Source
	}
	if override.Width != 0 {
		result.Width = override.Width
	}
	if override.Height != 0 {
		result.Height = override.Height
	}
	if override.X != 0 {
		result.X = override.X
	}
	if override.Y != 0 {
		result.Y = override.Y
	}
	if override.Template != "" {
		result.Template = override.Template
	}
	// AutoCrop: override wins if true (can't "unset" from template)
	if override.AutoCrop {
		result.AutoCrop = override.AutoCrop
	}
	if override.CropThreshold != 0 {
		result.CropThreshold = override.CropThreshold
	}
	if override.CropPadding != 0 {
		result.CropPadding = override.CropPadding
	}
	return &result
}

// mergeDevicesArray handles merging of devices arrays from template and screen
// Priority: screen.Devices > template.Devices > combined single devices
func mergeDevicesArray(tmplDevices, screenDevices []DeviceImage, tmplDevice, screenDevice *DeviceImage) []DeviceImage {
	// If screen explicitly defines devices array, use it
	if len(screenDevices) > 0 {
		return screenDevices
	}
	// If template defines devices array, copy it
	if len(tmplDevices) > 0 {
		result := make([]DeviceImage, len(tmplDevices))
		copy(result, tmplDevices)
		return result
	}
	// Otherwise, return nil (single device mode via Device field)
	return nil
}

// resolvePath resolves a path relative to baseDir if it's not absolute
func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	// Join and clean the path to resolve ".." properly
	joined := filepath.Join(baseDir, path)
	// Convert to absolute path to handle ".." correctly
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}

// validate checks the configuration for errors
func validate(cfg *Config) error {
	if len(cfg.Devices) == 0 {
		return fmt.Errorf("no devices defined")
	}

	if len(cfg.Screens) == 0 {
		return fmt.Errorf("no screens defined")
	}

	if len(cfg.Outputs) == 0 {
		return fmt.Errorf("no outputs defined")
	}

	// Validate devices
	for name, device := range cfg.Devices {
		if device.Width <= 0 || device.Height <= 0 {
			return fmt.Errorf("device %q has invalid dimensions", name)
		}
	}

	// Build screen ID map for validation
	screenIDs := make(map[string]bool)
	for _, screen := range cfg.Screens {
		if screen.ID == "" {
			return fmt.Errorf("screen has empty ID")
		}
		if screenIDs[screen.ID] {
			return fmt.Errorf("duplicate screen ID: %s", screen.ID)
		}
		screenIDs[screen.ID] = true

		if screen.Background == nil {
			return fmt.Errorf("screen %q: missing background", screen.ID)
		}
		if err := validateBackground(screen.Background); err != nil {
			return fmt.Errorf("screen %q: %w", screen.ID, err)
		}

		// Validate device(s) - either single device or devices array
		if len(screen.Devices) > 0 {
			for i, dev := range screen.Devices {
				if err := validateDeviceImage(&dev); err != nil {
					return fmt.Errorf("screen %q devices[%d]: %w", screen.ID, i, err)
				}
			}
		} else if screen.Device != nil {
			if err := validateDeviceImage(screen.Device); err != nil {
				return fmt.Errorf("screen %q: %w", screen.ID, err)
			}
		} else {
			return fmt.Errorf("screen %q: missing device or devices", screen.ID)
		}
	}

	// Validate outputs
	for i, output := range cfg.Outputs {
		if _, ok := cfg.Devices[output.Device]; !ok {
			return fmt.Errorf("output %d references unknown device: %s", i, output.Device)
		}

		for _, screenID := range output.Screens {
			if !screenIDs[screenID] {
				return fmt.Errorf("output %d references unknown screen: %s", i, screenID)
			}
		}
	}

	return nil
}

func validateBackground(bg *Background) error {
	switch bg.Type {
	case "solid":
		if bg.Color == "" {
			return fmt.Errorf("solid background requires color")
		}
	case "gradient":
		if len(bg.Stops) < 2 {
			return fmt.Errorf("gradient requires at least 2 stops")
		}
	default:
		return fmt.Errorf("unknown background type: %s", bg.Type)
	}
	return nil
}

func validateDeviceImage(di *DeviceImage) error {
	if di.Source == "" {
		return fmt.Errorf("device requires source")
	}
	return nil
}

// GetScreen finds a screen by ID
func (c *Config) GetScreen(id string) *Screen {
	for i := range c.Screens {
		if c.Screens[i].ID == id {
			return &c.Screens[i]
		}
	}
	return nil
}
