package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/mcpserver"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP stdio server",
		Long: `Expose marina SSH hosts as MCP tools over stdio.

Add to your Claude Desktop / Claude Code config:
  {
    "mcpServers": {
      "marina": { "command": "marina", "args": ["mcp"] }
    }
  }

Config file: $XDG_CONFIG_HOME/marina/mcp.json
  {
    "allowed_hosts":         ["*"],
    "confirm_destructive":   false,
    "default_timeout_secs":  30,
    "max_output_bytes":      65536,
    "audit_log":             ""
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			v, err := openVault()
			if err != nil {
				return err
			}

			dir, err := configDir()
			if err != nil {
				return err
			}
			cfg, err := mcpserver.LoadConfig(filepath.Join(dir, "mcp.json"))
			if err != nil {
				return err
			}

			return mcpserver.New(st, v, cfg).Serve(ctx)
		},
	}
}
