package syncserver

import (
	"context"
	"errors"

	syncp "github.com/Yue2u/marina/internal/core/sync"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("username already taken")
	ErrVersionConflict = errors.New("version conflict")
)

// User — зарегистрированный пользователь.
type User struct {
	ID           string
	Username     string
	PasswordHash string // bcrypt
}

// Storage — абстрактное хранилище для sync-сервера.
// Работает поверх любого database/sql-совместимого драйвера (SQLite, Postgres, ...).
type Storage interface {
	// Пользователи
	CreateUser(ctx context.Context, id, username, passwordHash string) error
	GetUser(ctx context.Context, username string) (User, error)

	// Сессии
	CreateSession(ctx context.Context, token, userID string) error
	GetUserByToken(ctx context.Context, token string) (User, error)
	DeleteSession(ctx context.Context, token string) error

	// Снапшоты (per-user)
	GetSnapshot(ctx context.Context, userID string) (syncp.Snapshot, error)
	// PushSnapshot атомарно проверяет version и сохраняет новый блоб.
	// Возвращает ErrVersionConflict если текущая версия != baseVersion.
	PushSnapshot(ctx context.Context, userID string, baseVersion int, blob []byte) (syncp.Snapshot, error)
}
