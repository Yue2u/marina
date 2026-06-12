package sshx

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func KnownHostsCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	if err := ensureFile(path, 0600); err != nil {
		return nil, err
	}
	return tofuCallback(path), nil
}

// tofuCallback implements trust-on-first-use: unknown hosts prompt the user,
// key mismatches are rejected as a potential MITM.
func tofuCallback(path string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		cb, err := knownhosts.New(path)
		if err != nil {
			return err
		}
		err = cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("host key mismatch for %s — possible MITM attack", hostname)
		}

		// Unknown host — ask user
		fmt.Fprintf(os.Stderr, "Host %s not in known_hosts\nFingerprint: %s\nTrust? [y/N]: ",
			hostname, ssh.FingerprintSHA256(key))
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return fmt.Errorf("host key rejected")
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		// ensure the file ends with a newline before appending
		if info, err := f.Stat(); err == nil && info.Size() > 0 {
			buf := make([]byte, 1)
			if _, err := f.ReadAt(buf, info.Size()-1); err == nil && buf[0] != '\n' {
				f.WriteString("\n")
			}
		}
		_, err = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
		return err
	}
}

func ensureFile(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		return os.WriteFile(path, nil, perm)
	}
	return nil
}
