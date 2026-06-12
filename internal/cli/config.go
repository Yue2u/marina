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
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "marina")
	return dir, os.MkdirAll(dir, 0700)
}

func openStore() (*store.SQLiteStore, error) {
	dir, err := configDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	return store.OpenSQLite(filepath.Join(dir, "db.sqlite"))
}

// openVault открывает хранилище секретов.
//
// Бэкенд выбирается через MARINA_VAULT_BACKEND:
//
//	""/"file"   — зашифрованный файл ~/.config/marina/vault.enc (по умолчанию)
//	"hashicorp" — HashiCorp Vault KV v2; требует MARINA_VAULT_ADDR и MARINA_VAULT_TOKEN
//
// MARINA_VAULT_MOUNT задаёт KV mount path (по умолчанию "secret").
// MARINA_VAULT_PASSWORD пропускает интерактивный промпт.
func openVault() (*vault.Vault, error) {
	switch os.Getenv("MARINA_VAULT_BACKEND") {
	case "hashicorp":
		addr := os.Getenv("MARINA_VAULT_ADDR")
		token := os.Getenv("MARINA_VAULT_TOKEN")
		if addr == "" || token == "" {
			return nil, fmt.Errorf("MARINA_VAULT_ADDR and MARINA_VAULT_TOKEN required for hashicorp backend")
		}
		mount := os.Getenv("MARINA_VAULT_MOUNT")
		if mount == "" {
			mount = "secret"
		}
		// Мастер-пароль нужен для E2E-шифрования sync-дампа:
		// даже при HCV-бэкенде sync-блоб шифруется локально.
		masterPW := []byte(os.Getenv("MARINA_VAULT_PASSWORD"))
		if len(masterPW) == 0 {
			return nil, fmt.Errorf("MARINA_VAULT_PASSWORD required for sync encryption with hashicorp backend")
		}
		return vault.OpenHCVault(addr, token, mount, masterPW)

	default: // "file" или пусто
		dir, err := configDir()
		if err != nil {
			return nil, fmt.Errorf("config dir: %w", err)
		}
		return vault.Unlock(filepath.Join(dir, "vault.enc"))
	}
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
