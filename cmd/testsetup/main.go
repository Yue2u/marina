// testsetup seeds a test host into the marina store+vault.
// Usage: testsetup <vault-password> <ssh-password>
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/store"
	"github.com/Yue2u/marina/internal/core/vault"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: testsetup <vault-password> <ssh-password>")
		os.Exit(1)
	}
	vaultPw := []byte(os.Args[1])
	sshPw := []byte(os.Args[2])

	cfgDir, _ := os.UserConfigDir()
	dir := filepath.Join(cfgDir, "marina")
	os.MkdirAll(dir, 0700)

	s, err := store.OpenSQLite(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "store:", err)
		os.Exit(1)
	}
	defer s.Close()

	v, err := vault.UnlockWithPassword(filepath.Join(dir, "vault.enc"), vaultPw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vault:", err)
		os.Exit(1)
	}

	id := uuid.New().String()
	ref := "host:" + id + ":password"

	h := core.Host{
		ID:        id,
		Label:     "test-server",
		Hostname:  "109.172.94.181",
		Port:      22,
		Username:  "root",
		Auth:      core.AuthPassword,
		SecretRef: ref,
		UpdatedAt: time.Now(),
	}

	if err := s.SaveHost(context.Background(), h); err != nil {
		fmt.Fprintln(os.Stderr, "save host:", err)
		os.Exit(1)
	}
	if err := v.Set(ref, sshPw); err != nil {
		fmt.Fprintln(os.Stderr, "vault set:", err)
		os.Exit(1)
	}

	fmt.Printf("OK: added %q (id=%s)\n", h.Label, h.ID)
}
