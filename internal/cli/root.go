package cli

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "marina",
		Short: "Dock to any server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchTUI()
		},
	}
	root.AddCommand(
		connectCmd(),
		lsCmd(),
		addCmd(),
		rmCmd(),
		mvCmd(),
		folderCmd(),
		importCmd(),
		receiveCmd(),
		syncCmd(),
		serveCmd(),
		mcpCmd(),
	)
	return root
}
