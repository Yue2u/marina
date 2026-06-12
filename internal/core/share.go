package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const sharePrefix = "marina+v1+"

// sharePayload — минимальный набор полей для шаринга.
// Пароли никогда не включаются; для AuthPassword поле SecretRef пустое.
type sharePayload struct {
	Label    string `json:"l"`
	Hostname string `json:"h"`
	Port     int    `json:"p"`
	Username string `json:"u"`
	Auth     string `json:"a"`
	KeyPath  string `json:"k,omitempty"` // только для AuthKey
}

// EncodeShare кодирует хост в shareable-код вида "marina+v1+<base64url>".
// Секреты (пароли, passphrase) не включаются.
func EncodeShare(h Host) string {
	pl := sharePayload{
		Label:    h.Label,
		Hostname: h.Hostname,
		Port:     h.Port,
		Username: h.Username,
		Auth:     string(h.Auth),
	}
	if h.Auth == AuthKey {
		pl.KeyPath = h.SecretRef
	}
	raw, _ := json.Marshal(pl)
	return sharePrefix + base64.URLEncoding.EncodeToString(raw)
}

// DecodeShare декодирует код полученный от EncodeShare в Host с новым UUID.
func DecodeShare(code string) (Host, error) {
	b64, ok := strings.CutPrefix(code, sharePrefix)
	if !ok {
		return Host{}, fmt.Errorf("не является share-кодом marina (должен начинаться с %q)", sharePrefix)
	}
	raw, err := base64.URLEncoding.DecodeString(b64)
	if err != nil {
		return Host{}, fmt.Errorf("decode share: %w", err)
	}
	var pl sharePayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return Host{}, fmt.Errorf("parse share: %w", err)
	}
	auth := AuthType(pl.Auth)
	if auth != AuthPassword && auth != AuthKey {
		auth = AuthAgent
	}
	port := pl.Port
	if port == 0 {
		port = 22
	}
	return Host{
		ID:        uuid.New().String(),
		Label:     pl.Label,
		Hostname:  pl.Hostname,
		Port:      port,
		Username:  pl.Username,
		Auth:      auth,
		SecretRef: pl.KeyPath,
		UpdatedAt: time.Now(),
	}, nil
}
