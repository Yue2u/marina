package sync

import "github.com/Yue2u/marina/internal/core"

// Dump — всё что шифруется и отправляется на сервер.
// Включает tombstones (DeletedAt != nil) чтобы удаления тоже синхронизировались.
type Dump struct {
	Hosts   []core.Host   `json:"hosts"`
	Folders []core.Folder `json:"folders"`
}

// Snapshot — то что хранит сервер и отдаёт клиенту.
type Snapshot struct {
	Version int    `json:"version"` // монотонный счётчик
	Blob    []byte `json:"blob"`    // зашифрованный Dump (base64 в JSON)
}

// PushRequest — запрос от клиента на сохранение нового снапшота.
type PushRequest struct {
	BaseVersion int    `json:"base_version"` // версия от которой клиент отталкивался
	Blob        []byte `json:"blob"`
}

// AuthRequest — запрос токена.
type AuthRequest struct {
	Token string `json:"token"`
}

// AuthResponse — ответ с сессионным токеном (на будущее; сейчас токен = Bearer напрямую).
type AuthResponse struct {
	OK bool `json:"ok"`
}
