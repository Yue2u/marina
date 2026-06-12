package cli

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/core"
)

func folderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folder",
		Short: "Manage folders",
	}
	cmd.AddCommand(folderAddCmd(), folderRmCmd())
	return cmd
}

func folderAddCmd() *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			f := core.Folder{
				ID:   uuid.New().String(),
				Name: args[0],
			}

			if parent != "" {
				folders, err := s.Folders(ctx)
				if err != nil {
					return err
				}
				for _, existing := range folders {
					if existing.ID == parent || existing.Name == parent {
						f.ParentID = &existing.ID
						break
					}
				}
				if f.ParentID == nil {
					return fmt.Errorf("parent folder %q not found", parent)
				}
			}

			if err := s.SaveFolder(ctx, f); err != nil {
				return err
			}
			fmt.Printf("Created folder %q (%s)\n", f.Name, f.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Parent folder name or id")
	return cmd
}

func folderRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <folder>",
		Short: "Delete a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			folders, err := s.Folders(ctx)
			if err != nil {
				return err
			}
			var id string
			for _, f := range folders {
				if f.ID == args[0] || f.Name == args[0] {
					id = f.ID
					break
				}
			}
			if id == "" {
				return fmt.Errorf("folder %q not found", args[0])
			}

			s.DeleteFolder(ctx, id)
			fmt.Printf("Deleted folder %q\n", args[0])
			return nil
		},
	}
}
