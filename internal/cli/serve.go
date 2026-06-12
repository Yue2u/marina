package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a sync server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("serve is not yet implemented (coming in stage 3)")
		},
	}
	cmd.Flags().String("addr", ":8443", "Listen address")
	cmd.Flags().String("data", "/var/lib/marina", "Data directory")
	return cmd
}
