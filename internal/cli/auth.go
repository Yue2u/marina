package cli

import (
	"golang.org/x/crypto/ssh"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/sshx"
	"github.com/Yue2u/marina/internal/core/vault"
)

func buildAuth(h core.Host, v *vault.Vault) ([]ssh.AuthMethod, error) {
	return sshx.BuildAuth(h, v)
}
