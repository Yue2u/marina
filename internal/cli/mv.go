package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func mvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <host> <folder>",
		Short: "Move a host to a folder",
		Args:  cobra.ExactArgs(2),
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

			folders, err := s.Folders(ctx)
			if err != nil {
				return err
			}
			var folderID *string
			for _, f := range folders {
				if f.ID == args[1] || f.Name == args[1] {
					folderID = &f.ID
					break
				}
			}
			if folderID == nil {
				return fmt.Errorf("folder %q not found", args[1])
			}

			h.FolderID = folderID
			h.UpdatedAt = time.Now()
			if err := s.SaveHost(ctx, h); err != nil {
				return err
			}
			fmt.Printf("Moved %q to folder %q\n", h.Label, args[1])
			return nil
		},
	}
}
