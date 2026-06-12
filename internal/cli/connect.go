package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/sshx"
	"github.com/Yue2u/marina/internal/core/vault"
)

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <host>",
		Short: "Connect to a host by label or id",
		Args:  cobra.ExactArgs(1),
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

			var v *vault.Vault
			if h.Auth != core.AuthAgent {
				v, err = openVault()
				if err != nil {
					return err
				}
			}

			authMethods, err := buildAuth(h, v)
			if err != nil {
				return err
			}

			cb, err := sshx.KnownHostsCallback()
			if err != nil {
				return fmt.Errorf("known hosts: %w", err)
			}

			creds := sshx.Creds{User: h.Username, Auth: authMethods}

			var client *ssh.Client
			if h.JumpHostID != nil {
				jump, err := findHost(ctx, s, *h.JumpHostID)
				if err != nil {
					return fmt.Errorf("jump host: %w", err)
				}
				jumpAuth, err := buildAuth(jump, v)
				if err != nil {
					return fmt.Errorf("jump host auth: %w", err)
				}
				jumpClient, err := sshx.Dial(jump.Addr(), sshx.Creds{User: jump.Username, Auth: jumpAuth}, cb)
				if err != nil {
					return fmt.Errorf("connect to jump host: %w", err)
				}
				defer jumpClient.Close()
				client, err = sshx.DialJump(jumpClient, h.Addr(), creds, cb)
			} else {
				client, err = sshx.Dial(h.Addr(), creds, cb)
			}
			if err != nil {
				return fmt.Errorf("connect to %s: %w", h.Addr(), err)
			}
			defer client.Close()

			return sshx.InteractiveShell(client)
		},
	}
}
