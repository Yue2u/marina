package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <host>",
		Short: "Remove a host by label or id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			h, err := findHost(ctx, s, args[0])
			if err != nil {
				return err
			}
			if err := s.DeleteHost(ctx, h.ID); err != nil {
				return err
			}
			fmt.Printf("Removed %q\n", h.Label)
			return nil
		},
	}
}
