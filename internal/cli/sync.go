package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/core"
	syncp "github.com/Yue2u/marina/internal/core/sync"
	"github.com/Yue2u/marina/internal/core/vault"
)

func syncCmd() *cobra.Command {
	var url, token string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize with your sync server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				url = os.Getenv("MARINA_SYNC_URL")
			}
			if token == "" {
				token = os.Getenv("MARINA_SYNC_TOKEN")
			}
			if url == "" {
				return fmt.Errorf("--url is required (or set MARINA_SYNC_URL)")
			}
			if token == "" {
				return fmt.Errorf("--token is required (or set MARINA_SYNC_TOKEN)")
			}
			return runSync(cmd.Context(), url, token)
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "Sync server URL")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token")
	return cmd
}

func runSync(ctx context.Context, url, token string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	v, err := openVault()
	if err != nil {
		return err
	}

	client := syncp.NewClient(url, token)

	// pull текущего снапшота с сервера, чтобы знать base_version
	remote, err := client.Pull(ctx)
	if err != nil {
		return fmt.Errorf("pull: %w", err)
	}

	// зашифровать локальный дамп
	blob, err := encryptDump(ctx, st, v)
	if err != nil {
		return fmt.Errorf("encrypt dump: %w", err)
	}

	baseVersion := remote.Version

	// push → при 409: merge и повторить (максимум 3 попытки)
	for attempt := range 3 {
		newSnap, pushErr := client.Push(ctx, baseVersion, blob)
		if pushErr == nil {
			fmt.Printf("synced (version %d → %d)\n", baseVersion, newSnap.Version)
			return nil
		}
		if !errors.Is(pushErr, syncp.ErrConflict) {
			return pushErr
		}

		fmt.Printf("conflict (attempt %d) — pulling and merging...\n", attempt+1)

		remote, err = client.Pull(ctx)
		if err != nil {
			return fmt.Errorf("pull after conflict: %w", err)
		}

		if err := applyAndMerge(ctx, st, v, remote); err != nil {
			return fmt.Errorf("merge: %w", err)
		}

		blob, err = encryptDump(ctx, st, v)
		if err != nil {
			return fmt.Errorf("re-encrypt after merge: %w", err)
		}
		baseVersion = remote.Version
	}

	return fmt.Errorf("sync failed after 3 attempts")
}

// encryptDump сериализует все хосты+папки (включая tombstones) и шифрует vault-паролем.
func encryptDump(ctx context.Context, st interface {
	HostsAll(context.Context) ([]core.Host, error)
	Folders(context.Context) ([]core.Folder, error)
}, v *vault.Vault) ([]byte, error) {
	hosts, err := st.HostsAll(ctx)
	if err != nil {
		return nil, err
	}
	folders, err := st.Folders(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(syncp.Dump{Hosts: hosts, Folders: folders})
	if err != nil {
		return nil, err
	}
	return vault.Seal(v.Password(), raw)
}

// applyAndMerge расшифровывает удалённый снапшот, мержит с локальным store и сохраняет результат.
func applyAndMerge(ctx context.Context, st interface {
	HostsAll(context.Context) ([]core.Host, error)
	Folders(context.Context) ([]core.Folder, error)
	SaveHost(context.Context, core.Host) error
	SaveFolder(context.Context, core.Folder) error
}, v *vault.Vault, remote *syncp.Snapshot) error {
	if len(remote.Blob) == 0 {
		return nil
	}

	plaintext, err := vault.Open(v.Password(), remote.Blob)
	if err != nil {
		return fmt.Errorf("decrypt remote: %w", err)
	}

	var remoteDump syncp.Dump
	if err := json.Unmarshal(plaintext, &remoteDump); err != nil {
		return fmt.Errorf("decode remote dump: %w", err)
	}

	localHosts, err := st.HostsAll(ctx)
	if err != nil {
		return err
	}
	localFolders, err := st.Folders(ctx)
	if err != nil {
		return err
	}

	mergedHosts := syncp.MergeHosts(localHosts, remoteDump.Hosts)
	mergedFolders := syncp.MergeFolders(localFolders, remoteDump.Folders)

	for _, h := range mergedHosts {
		if err := st.SaveHost(ctx, h); err != nil {
			return fmt.Errorf("save merged host %q: %w", h.ID, err)
		}
	}
	for _, f := range mergedFolders {
		if err := st.SaveFolder(ctx, f); err != nil {
			return fmt.Errorf("save merged folder %q: %w", f.ID, err)
		}
	}
	return nil
}
