package cmd

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/philipparndt/delivr/internal/editor"
)

var (
	editConfig   string
	editPort     int
	editOpen     bool
	editReadOnly bool
)

var editCmd = &cobra.Command{
	Use:          "edit",
	Short:        "Position devices and copy visually, with a live preview",
	Long:         GetHelp("edit"),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := editConfig
		if cfg == "" {
			if rootFlag := rootCmd.Flags().Lookup("config"); rootFlag != nil && rootFlag.Changed {
				cfg = rootFlag.Value.String()
			}
		}
		if cfg == "" {
			return fmt.Errorf("--config is required")
		}

		srv, err := editor.New(cfg, editor.Options{
			Port:     editPort,
			ReadOnly: editReadOnly,
			Open:     editOpen,
		})
		if err != nil {
			return err
		}
		defer srv.Close()

		// Ctrl-C stops the server rather than killing the process mid-write.
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		return srv.Serve(ctx, editor.Options{
			Port:     editPort,
			ReadOnly: editReadOnly,
			Open:     editOpen,
		})
	},
}

func init() {
	editCmd.Flags().StringVar(&editConfig, "config", "", "Path to configuration YAML file")
	editCmd.Flags().IntVar(&editPort, "port", 0, "Port to listen on (0 picks a free one)")
	editCmd.Flags().BoolVar(&editOpen, "open", true, "Open the editor in a browser")
	editCmd.Flags().BoolVar(&editReadOnly, "read-only", false, "Never write to the config; copy values instead")
	rootCmd.AddCommand(editCmd)
}
