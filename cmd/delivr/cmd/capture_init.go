package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed snapshot/SnapshotHelper.swift
var snapshotHelperSwift string

var captureInitOutput string

var captureInitCmd = &cobra.Command{
	Use:          "init",
	Short:        "Generate SnapshotHelper.swift for your UI test target",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if captureInitOutput == "" {
			captureInitOutput = "SnapshotHelper.swift"
		}

		// Ensure parent directory exists
		dir := filepath.Dir(captureInitOutput)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if err := os.WriteFile(captureInitOutput, []byte(snapshotHelperSwift), 0644); err != nil {
			return fmt.Errorf("failed to write SnapshotHelper.swift: %w", err)
		}

		fmt.Printf("Created %s\n", captureInitOutput)
		fmt.Println("Add this file to your UI test target in Xcode.")
		return nil
	},
}

func init() {
	captureInitCmd.Flags().StringVarP(&captureInitOutput, "output", "o", "", "Output path (default: ./SnapshotHelper.swift)")
	captureCmd.AddCommand(captureInitCmd)
}
