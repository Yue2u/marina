package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Yue2u/marina/internal/core"
)

func addCmd() *cobra.Command {
	var (
		hostname string
		username string
		port     int
		authType string
		keyPath  string
		folder   string
		jump     string
		tags     []string
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			id := uuid.New().String()
			h := core.Host{
				ID:        id,
				Label:     args[0],
				Hostname:  hostname,
				Port:      port,
				Username:  username,
				Auth:      core.AuthType(authType),
				Tags:      tags,
				UpdatedAt: time.Now(),
			}

			if folder != "" {
				folders, err := s.Folders(ctx)
				if err != nil {
					return err
				}
				for _, f := range folders {
					if f.ID == folder || f.Name == folder {
						h.FolderID = &f.ID
						break
					}
				}
				if h.FolderID == nil {
					return fmt.Errorf("folder %q not found", folder)
				}
			}

			if jump != "" {
				jh, err := findHost(ctx, s, jump)
				if err != nil {
					return fmt.Errorf("jump host: %w", err)
				}
				h.JumpHostID = &jh.ID
			}

			switch h.Auth {
			case core.AuthPassword:
				h.SecretRef = "host:" + id + ":password"
				v, err := openVault()
				if err != nil {
					return err
				}
				fmt.Fprint(os.Stderr, "SSH password: ")
				pw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return err
				}
				if err := v.Set(h.SecretRef, pw); err != nil {
					return err
				}

			case core.AuthKey:
				if keyPath == "" {
					return fmt.Errorf("--key-path required for --auth key")
				}
				if strings.HasPrefix(keyPath, "~/") {
					home, _ := os.UserHomeDir()
					keyPath = home + keyPath[1:]
				}
				h.SecretRef = keyPath

				fmt.Fprint(os.Stderr, "Key passphrase (empty if none): ")
				pp, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return err
				}
				if len(pp) > 0 {
					v, err := openVault()
					if err != nil {
						return err
					}
					if err := v.Set("host:"+id+":passphrase", pp); err != nil {
						return err
					}
				}

			case core.AuthAgent:
				// nothing to store

			default:
				return fmt.Errorf("unknown auth type %q (valid: password, key, agent)", authType)
			}

			if err := s.SaveHost(ctx, h); err != nil {
				return err
			}
			fmt.Printf("Added %q (%s)\n", h.Label, h.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "host", "H", "", "Hostname or IP (required)")
	cmd.Flags().StringVarP(&username, "user", "u", "", "SSH username (required)")
	cmd.Flags().IntVarP(&port, "port", "p", 22, "SSH port")
	cmd.Flags().StringVar(&authType, "auth", "agent", "Auth type: password, key, agent")
	cmd.Flags().StringVar(&keyPath, "key-path", "", "Path to private key (for --auth key)")
	cmd.Flags().StringVar(&folder, "folder", "", "Folder name or id")
	cmd.Flags().StringVar(&jump, "jump", "", "Jump host label or id")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag (repeatable)")

	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("user")

	return cmd
}
