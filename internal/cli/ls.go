package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/core"
)

func lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all folders and hosts",
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
			hosts, err := s.Hosts(ctx, nil)
			if err != nil {
				return err
			}

			byFolder := map[string][]core.Host{}
			var noFolder []core.Host
			for _, h := range hosts {
				if h.FolderID != nil {
					byFolder[*h.FolderID] = append(byFolder[*h.FolderID], h)
				} else {
					noFolder = append(noFolder, h)
				}
			}

			for _, f := range folders {
				fmt.Printf("[%s]\n", f.Name)
				for _, h := range byFolder[f.ID] {
					fmt.Printf("  %-24s  %s@%s\n", h.Label, h.Username, h.Addr())
				}
			}
			for _, h := range noFolder {
				fmt.Printf("%-24s  %s@%s\n", h.Label, h.Username, h.Addr())
			}
			return nil
		},
	}
}
