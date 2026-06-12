package cli

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/vault"
)

// buildAuth constructs SSH auth methods from host config and vault.
// vault may be nil for agent auth.
func buildAuth(h core.Host, v *vault.Vault) ([]ssh.AuthMethod, error) {
	switch h.Auth {
	case core.AuthPassword:
		if v == nil {
			return nil, fmt.Errorf("vault required for password auth")
		}
		pw, err := v.Get(h.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("get password from vault: %w", err)
		}
		return []ssh.AuthMethod{ssh.Password(string(pw))}, nil

	case core.AuthKey:
		keyBytes, err := os.ReadFile(h.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("read key file %q: %w", h.SecretRef, err)
		}
		// check vault for passphrase
		if v != nil {
			if pp, _ := v.Get("host:" + h.ID + ":passphrase"); pp != nil {
				signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, pp)
				if err != nil {
					return nil, fmt.Errorf("parse key with passphrase: %w", err)
				}
				return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
			}
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil

	case core.AuthAgent:
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, fmt.Errorf("connect to ssh-agent: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(conn).Signers)}, nil

	default:
		return nil, fmt.Errorf("unknown auth type: %s", h.Auth)
	}
}
