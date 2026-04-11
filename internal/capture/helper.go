package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type helperConfig struct {
	DeviceName string `json:"device_name"`
	OutputDir  string `json:"output_dir"`
	WindowSize [2]int `json:"window_size,omitempty"` // [width, height] for macOS window sizing
}

// WriteHelperConfig writes a per-device JSON config file to the host Mac's
// caches directory. Each simulator gets its own config file keyed by UDID
// so parallel runs don't overwrite each other.
func WriteHelperConfig(udid, deviceName, outputDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	cachesDir := filepath.Join(home, "Library", "Caches", "tools.delivr")
	return writeConfig(cachesDir, udid, deviceName, outputDir, [2]int{})
}

// WriteHelperConfigMacOS writes the config to the host Mac's caches directory.
func WriteHelperConfigMacOS(deviceName, outputDir string, windowSize [2]int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	cachesDir := filepath.Join(home, "Library", "Caches", "tools.delivr")
	return writeConfig(cachesDir, "macos", deviceName, outputDir, windowSize)
}

func writeConfig(cachesDir, udid, deviceName, outputDir string, windowSize [2]int) error {
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		return fmt.Errorf("failed to create caches dir: %w", err)
	}

	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output dir: %w", err)
	}

	cfg := helperConfig{
		DeviceName: deviceName,
		OutputDir:  absOutput,
		WindowSize: windowSize,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal helper config: %w", err)
	}

	configPath := filepath.Join(cachesDir, "snapshot-config-"+udid+".json")
	return os.WriteFile(configPath, data, 0644)
}
