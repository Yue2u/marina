package vault

// Backend — хранилище секретов. Два готовых бэкенда:
//   - fileBackend  — локальный зашифрованный файл (Argon2id + XChaCha20-Poly1305)
//   - hcVaultBackend — HashiCorp Vault KV v2 (без дополнительных зависимостей, чистый HTTP)
type Backend interface {
	Get(ref string) ([]byte, error)
	Set(ref string, value []byte) error
	Delete(ref string) error
}
