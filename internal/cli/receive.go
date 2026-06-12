package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/core"
)

func receiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "receive <code>",
		Short: "Import a host from a share code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := core.DecodeShare(args[0])
			if err != nil {
				return fmt.Errorf("invalid share code: %w", err)
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.SaveHost(cmd.Context(), h); err != nil {
				return fmt.Errorf("save host: %w", err)
			}

			fmt.Printf("imported %q (%s@%s:%d, auth: %s)\n",
				h.Label, h.Username, h.Hostname, h.Port, h.Auth)
			return nil
		},
	}
}
