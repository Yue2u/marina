package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [push|pull]",
		Short: "Synchronize with your sync server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("sync is not yet implemented (coming in stage 3)")
		},
	}
}
