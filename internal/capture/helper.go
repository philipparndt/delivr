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
}

// WriteHelperConfig writes a JSON config file to the host Mac's caches
// directory. The SnapshotHelper.swift reads this at test runtime using
// the SIMULATOR_HOST_HOME environment variable (same approach as fastlane).
func WriteHelperConfig(udid, deviceName, outputDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	cachesDir := filepath.Join(home, "Library", "Caches", "tools.delivr")
	return writeConfig(cachesDir, deviceName, outputDir)
}

// WriteHelperConfigMacOS writes the config to the host Mac's caches directory.
func WriteHelperConfigMacOS(deviceName, outputDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	cachesDir := filepath.Join(home, "Library", "Caches", "tools.delivr")
	return writeConfig(cachesDir, deviceName, outputDir)
}

func writeConfig(cachesDir, deviceName, outputDir string) error {
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
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal helper config: %w", err)
	}

	configPath := filepath.Join(cachesDir, "snapshot-config.json")
	return os.WriteFile(configPath, data, 0644)
}
