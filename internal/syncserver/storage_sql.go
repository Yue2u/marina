package syncserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	syncp "github.com/Yue2u/marina/internal/core/sync"
)

// SQLStorage реализует Storage поверх database/sql.
// Поддерживает SQLite ("sqlite") и PostgreSQL ("postgres") через placeholder rebinding.
//
// Postgres: зарегистрируй драйвер в вызывающем коде:
//
//	import _ "github.com/jackc/pgx/v5/stdlib"
//	db, _ := sql.Open("pgx", dsn)
type SQLStorage struct {
	db      *sql.DB
	dialect string // "sqlite" | "postgres"
}

// NewSQLStorage создаёт хранилище поверх уже открытого *sql.DB.
// dialect влияет только на синтаксис placeholder'ов (? vs $N).
func NewSQLStorage(db *sql.DB, dialect string) (*SQLStorage, error) {
	s := &SQLStorage{db: db, dialect: dialect}
	if err := s.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("migrate sync storage: %w", err)
	}
	return s, nil
}

// rebind заменяет ? на $1, $2, ... для PostgreSQL.
func (s *SQLStorage) rebind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	n := 1
	for _, c := range query {
		if c == '?' {
			fmt.Fprintf(&b, "$%d", n)
			n++
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s *SQLStorage) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS snapshots (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 0,
    blob    BLOB
);
`)
	return err
}

// ── Users ─────────────────────────────────────────────────────────────────────

func (s *SQLStorage) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`),
		id, username, passwordHash,
	)
	if err != nil && isUniqueViolation(err) {
		return ErrUserExists
	}
	return err
}

func (s *SQLStorage) GetUser(ctx context.Context, username string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		s.rebind(`SELECT id, username, password_hash FROM users WHERE username = ?`),
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrUserNotFound
	}
	return u, err
}

// ── Sessions ──────────────────────────────────────────────────────────────────

func (s *SQLStorage) CreateSession(ctx context.Context, token, userID string) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`INSERT INTO sessions (token, user_id, created_at) VALUES (?, ?, ?)`),
		token, userID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *SQLStorage) GetUserByToken(ctx context.Context, token string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		s.rebind(`SELECT u.id, u.username, u.password_hash
		          FROM sessions s JOIN users u ON s.user_id = u.id
		          WHERE s.token = ?`),
		token,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrUserNotFound
	}
	return u, err
}

func (s *SQLStorage) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`DELETE FROM sessions WHERE token = ?`),
		token,
	)
	return err
}

// ── Snapshots ─────────────────────────────────────────────────────────────────

func (s *SQLStorage) GetSnapshot(ctx context.Context, userID string) (syncp.Snapshot, error) {
	var snap syncp.Snapshot
	var blob []byte
	err := s.db.QueryRowContext(ctx,
		s.rebind(`SELECT version, blob FROM snapshots WHERE user_id = ?`),
		userID,
	).Scan(&snap.Version, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return syncp.Snapshot{}, nil // нет снапшота — Version=0
	}
	if err != nil {
		return snap, err
	}
	snap.Blob = blob
	return snap, nil
}

func (s *SQLStorage) PushSnapshot(ctx context.Context, userID string, baseVersion int, blob []byte) (syncp.Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return syncp.Snapshot{}, err
	}
	defer tx.Rollback()

	// читаем текущую версию внутри транзакции
	var current int
	err = tx.QueryRowContext(ctx,
		s.rebind(`SELECT version FROM snapshots WHERE user_id = ?`),
		userID,
	).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return syncp.Snapshot{}, err
	}

	if current != baseVersion {
		return syncp.Snapshot{}, ErrVersionConflict
	}

	newVersion := current + 1
	_, err = tx.ExecContext(ctx,
		s.rebind(`INSERT INTO snapshots (user_id, version, blob)
		          VALUES (?, ?, ?)
		          ON CONFLICT(user_id) DO UPDATE SET version=excluded.version, blob=excluded.blob`),
		userID, newVersion, blob,
	)
	if err != nil {
		return syncp.Snapshot{}, err
	}

	if err := tx.Commit(); err != nil {
		return syncp.Snapshot{}, err
	}
	return syncp.Snapshot{Version: newVersion, Blob: blob}, nil
}

// isUniqueViolation проверяет типичные коды уникальных нарушений для SQLite и Postgres.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // SQLite
		strings.Contains(msg, "unique constraint") || // Postgres (pgx)
		strings.Contains(msg, "duplicate key") // Postgres (lib/pq)
}
