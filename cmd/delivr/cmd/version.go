package cmd

import (
	"fmt"

	"github.com/philipparndt/delivr/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  GetHelp("version"),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("delivr %s (commit: %s, built: %s)\n", version.Version, version.GitCommit, version.BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
