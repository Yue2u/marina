package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("mcp server is not yet implemented (coming in stage 4)")
		},
	}
}
