package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/store"
	"github.com/Yue2u/marina/internal/core/vault"
)

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".marina")
	return dir, os.MkdirAll(dir, 0700)
}

func openStore() (*store.SQLiteStore, error) {
	dir, err := configDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	return store.OpenSQLite(filepath.Join(dir, "marina.db"))
}

func openVault() (*vault.Vault, error) {
	dir, err := configDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	return vault.Unlock(filepath.Join(dir, "vault.enc"))
}

func findHost(ctx context.Context, s *store.SQLiteStore, labelOrID string) (core.Host, error) {
	hosts, err := s.Hosts(ctx, nil)
	if err != nil {
		return core.Host{}, err
	}
	for _, h := range hosts {
		if h.ID == labelOrID || h.Label == labelOrID {
			return h, nil
		}
	}
	return core.Host{}, fmt.Errorf("host %q not found", labelOrID)
}
