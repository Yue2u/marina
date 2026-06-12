package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// hcVaultBackend хранит секреты в HashiCorp Vault KV v2.
// Не требует официального SDK — используется только net/http.
//
// Секрет с ref "host:abc:password" хранится по пути <mount>/data/host/abc/password.
// Значение кодируется base64 и записывается в поле "v" внутри KV data.
type hcVaultBackend struct {
	addr   string // напр. "https://vault.example.com"
	token  string // Vault token (VAULT_TOKEN)
	mount  string // KV mount path, напр. "secret"
	client *http.Client
}

func newHCVaultBackend(addr, token, mount string) *hcVaultBackend {
	return &hcVaultBackend{
		addr:   strings.TrimRight(addr, "/"),
		token:  token,
		mount:  strings.Trim(mount, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// refToPath превращает ref "host:abc:password" → "host/abc/password"
func refToPath(ref string) string {
	return strings.ReplaceAll(ref, ":", "/")
}

func (b *hcVaultBackend) dataURL(ref string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s", b.addr, b.mount, refToPath(ref))
}

func (b *hcVaultBackend) metaURL(ref string) string {
	return fmt.Sprintf("%s/v1/%s/metadata/%s", b.addr, b.mount, refToPath(ref))
}

func (b *hcVaultBackend) Get(ref string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, b.dataURL(ref), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", b.token)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hcvault get %q: %w", ref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("secret %q not found", ref)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hcvault get %q: status %d", ref, resp.StatusCode)
	}

	// {"data": {"data": {"v": "<base64>"}, "metadata": {...}}}
	var envelope struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("hcvault get decode: %w", err)
	}
	encoded, ok := envelope.Data.Data["v"]
	if !ok {
		return nil, fmt.Errorf("secret %q: missing field 'v'", ref)
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func (b *hcVaultBackend) Set(ref string, value []byte) error {
	body, err := json.Marshal(map[string]any{
		"data": map[string]string{"v": base64.StdEncoding.EncodeToString(value)},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, b.dataURL(ref), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", b.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("hcvault set %q: %w", ref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("hcvault set %q: status %d", ref, resp.StatusCode)
	}
	return nil
}

func (b *hcVaultBackend) Delete(ref string) error {
	// DELETE /v1/{mount}/metadata/{path} удаляет все версии секрета
	req, err := http.NewRequest(http.MethodDelete, b.metaURL(ref), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", b.token)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("hcvault delete %q: %w", ref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("hcvault delete %q: status %d", ref, resp.StatusCode)
	}
	return nil
}
