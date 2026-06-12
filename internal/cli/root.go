package cli

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "marina",
		Short: "Dock to any server",
	}
	root.AddCommand(
		connectCmd(),
		lsCmd(),
		addCmd(),
		rmCmd(),
		mvCmd(),
		folderCmd(),
		importCmd(),
		syncCmd(),
		serveCmd(),
		mcpCmd(),
	)
	return root
}
