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

func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, 3, 64*1024, 4, chacha20poly1305.KeySize)
}

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

// Vault хранит секреты в зашифрованном файле.
// Весь map сериализуется в JSON и шифруется как один блоб.
type Vault struct {
	path     string
	password []byte
	secrets  map[string][]byte
}

// Unlock загружает vault из файла, запрашивая мастер-пароль.
// Если файл не существует — создаёт пустой vault.
func Unlock(path string) (*Vault, error) {
	fmt.Fprint(os.Stderr, "Master password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	v := &Vault{path: path, password: password, secrets: map[string][]byte{}}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return v, nil
	}
	if err != nil {
		return nil, err
	}

	plaintext, err := open(password, data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plaintext, &v.secrets); err != nil {
		return nil, fmt.Errorf("unmarshal vault: %w", err)
	}
	return v, nil
}

func (v *Vault) Get(ref string) ([]byte, error) {
	val, ok := v.secrets[ref]
	if !ok {
		return nil, fmt.Errorf("secret %q not found", ref)
	}
	return val, nil
}

func (v *Vault) Set(ref string, value []byte) error {
	v.secrets[ref] = value
	return v.save()
}

func (v *Vault) Delete(ref string) error {
	delete(v.secrets, ref)
	return v.save()
}

func (v *Vault) save() error {
	data, err := json.Marshal(v.secrets)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}
	blob, err := Seal(v.password, data)
	if err != nil {
		return err
	}
	return os.WriteFile(v.path, blob, 0600)
}

func open(password, blob []byte) ([]byte, error) {
	prefixSize := 16 + 24 // salt + nonce
	if len(blob) < prefixSize {
		return nil, errors.New("invalid password or corrupted vault")
	}

	salt := blob[:16]
	nonce := blob[16:prefixSize]
	ciphertext := blob[prefixSize:]

	key := deriveKey(password, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("invalid password or corrupted vault")
	}
	return plaintext, nil
}
