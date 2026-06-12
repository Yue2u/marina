package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/core"
)

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <ssh_config>",
		Short: "Import hosts from an SSH config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			entries, err := parseSSHConfig(args[0])
			if err != nil {
				return fmt.Errorf("parse ssh config: %w", err)
			}
			if len(entries) == 0 {
				fmt.Println("No hosts found in config.")
				return nil
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			for _, e := range entries {
				h := core.Host{
					ID:        uuid.New().String(),
					Label:     e.alias,
					Hostname:  e.hostname,
					Port:      e.port,
					Username:  e.user,
					Auth:      core.AuthAgent,
					UpdatedAt: time.Now(),
				}
				if e.identityFile != "" {
					h.Auth = core.AuthKey
					if strings.HasPrefix(e.identityFile, "~/") {
						home, _ := os.UserHomeDir()
						e.identityFile = home + e.identityFile[1:]
					}
					h.SecretRef = e.identityFile
				}
				// proxyJump wired after all hosts are imported (labels may differ)
				if err := s.SaveHost(ctx, h); err != nil {
					return fmt.Errorf("save %q: %w", e.alias, err)
				}
				fmt.Printf("  imported %q (%s@%s:%d)\n", e.alias, e.user, e.hostname, e.port)
			}
			fmt.Printf("Imported %d host(s).\n", len(entries))
			return nil
		},
	}
}

type sshConfigEntry struct {
	alias        string
	hostname     string
	user         string
	port         int
	identityFile string
	proxyJump    string
}

func parseSSHConfig(path string) ([]sshConfigEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []sshConfigEntry
	var cur *sshConfigEntry

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		val := strings.Join(parts[1:], " ")

		switch key {
		case "host":
			if cur != nil && cur.alias != "*" && cur.hostname != "" {
				entries = append(entries, *cur)
			}
			cur = &sshConfigEntry{alias: val, port: 22}
		case "hostname":
			if cur != nil {
				cur.hostname = val
			}
		case "user":
			if cur != nil {
				cur.user = val
			}
		case "port":
			if cur != nil {
				cur.port, _ = strconv.Atoi(val)
			}
		case "identityfile":
			if cur != nil {
				cur.identityFile = val
			}
		case "proxyjump":
			if cur != nil {
				cur.proxyJump = val
			}
		}
	}
	if cur != nil && cur.alias != "*" && cur.hostname != "" {
		entries = append(entries, *cur)
	}
	return entries, nil
}
