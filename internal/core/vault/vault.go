package vault

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/term"
)

// ── Crypto primitives (используются и файловым бэкендом, и sync-слоем) ──────

func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, 3, 64*1024, 4, chacha20poly1305.KeySize)
}

// Seal шифрует plaintext: salt(16) || nonce(24) || ciphertext.
func Seal(password, plaintext []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(salt, nonce...)
	return aead.Seal(out, nonce, plaintext, nil), nil
}

// Open расшифровывает блоб созданный через Seal.
func Open(password, blob []byte) ([]byte, error) {
	prefixSize := 16 + 24
	if len(blob) < prefixSize {
		return nil, errors.New("invalid password or corrupted vault")
	}
	key := deriveKey(password, blob[:16])
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, blob[16:prefixSize], blob[prefixSize:], nil)
	if err != nil {
		return nil, errors.New("invalid password or corrupted vault")
	}
	return plaintext, nil
}

// ── fileBackend — зашифрованный файл ─────────────────────────────────────────

type fileBackend struct {
	path     string
	password []byte
	secrets  map[string][]byte
}

func newFileBackend(path string, password []byte) (*fileBackend, error) {
	b := &fileBackend{path: path, password: password, secrets: map[string][]byte{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := Open(password, data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plaintext, &b.secrets); err != nil {
		return nil, fmt.Errorf("unmarshal vault: %w", err)
	}
	return b, nil
}

func (b *fileBackend) Get(ref string) ([]byte, error) {
	val, ok := b.secrets[ref]
	if !ok {
		return nil, fmt.Errorf("secret %q not found", ref)
	}
	return val, nil
}

func (b *fileBackend) Set(ref string, value []byte) error {
	b.secrets[ref] = value
	return b.save()
}

func (b *fileBackend) Delete(ref string) error {
	delete(b.secrets, ref)
	return b.save()
}

func (b *fileBackend) save() error {
	data, err := json.Marshal(b.secrets)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}
	blob, err := Seal(b.password, data)
	if err != nil {
		return err
	}
	return os.WriteFile(b.path, blob, 0600)
}

// ── Vault — тонкая обёртка над Backend ───────────────────────────────────────

// Vault хранит секреты через подключённый бэкенд.
// password всегда присутствует независимо от бэкенда — используется для
// шифрования sync-дампа (E2E), чтобы сервер sync не видел содержимое.
type Vault struct {
	backend  Backend
	password []byte
}

// Unlock открывает файловый бэкенд, запрашивая мастер-пароль.
// Переменная MARINA_VAULT_PASSWORD пропускает промпт.
func Unlock(path string) (*Vault, error) {
	password := []byte(os.Getenv("MARINA_VAULT_PASSWORD"))
	if len(password) == 0 {
		fmt.Fprint(os.Stderr, "Master password: ")
		var err error
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, err
		}
	}
	return UnlockWithPassword(path, password)
}

// UnlockWithPassword открывает файловый бэкенд с явным паролем.
func UnlockWithPassword(path string, password []byte) (*Vault, error) {
	b, err := newFileBackend(path, password)
	if err != nil {
		return nil, err
	}
	return &Vault{backend: b, password: password}, nil
}

// OpenHCVault открывает HashiCorp Vault как бэкенд.
// masterPassword нужен только для шифрования sync-дампа (E2E-свойство sync).
func OpenHCVault(addr, token, mount string, masterPassword []byte) (*Vault, error) {
	b := newHCVaultBackend(addr, token, mount)
	return &Vault{backend: b, password: masterPassword}, nil
}

func (v *Vault) Get(ref string) ([]byte, error)     { return v.backend.Get(ref) }
func (v *Vault) Set(ref string, value []byte) error { return v.backend.Set(ref, value) }
func (v *Vault) Delete(ref string) error            { return v.backend.Delete(ref) }
func (v *Vault) Password() []byte                   { return v.password }
