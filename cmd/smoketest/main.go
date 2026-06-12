// smoketest verifies the full path: store → vault → SSH dial → RunCommand.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/sshx"
	"github.com/Yue2u/marina/internal/core/store"
	"github.com/Yue2u/marina/internal/core/vault"
	"golang.org/x/crypto/ssh"
)

func main() {
	cfgDir, _ := os.UserConfigDir()
	dir := filepath.Join(cfgDir, "marina")

	s, err := store.OpenSQLite(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "store:", err)
		os.Exit(1)
	}
	defer s.Close()

	hosts, _ := s.Hosts(context.Background(), nil)
	var h core.Host
	for _, host := range hosts {
		if host.Label == "test-server" {
			h = host
		}
	}
	if h.ID == "" {
		fmt.Fprintln(os.Stderr, "test-server not found in store")
		os.Exit(1)
	}

	v, err := vault.UnlockWithPassword(filepath.Join(dir, "vault.enc"), []byte("test"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "vault:", err)
		os.Exit(1)
	}

	pw, err := v.Get(h.SecretRef)
	if err != nil {
		fmt.Fprintln(os.Stderr, "get secret:", err)
		os.Exit(1)
	}

	cb, err := sshx.KnownHostsCallback()
	if err != nil {
		fmt.Fprintln(os.Stderr, "knownhosts:", err)
		os.Exit(1)
	}

	client, err := sshx.Dial(h.Addr(), sshx.Creds{
		User: h.Username,
		Auth: []ssh.AuthMethod{ssh.Password(string(pw))},
	}, cb)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	defer client.Close()

	stdout, _, code, err := sshx.RunCommand(client, "uname -a && echo 'marina smoketest OK'")
	if err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	fmt.Printf("exit=%d\n%s", code, stdout)
}
