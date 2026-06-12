package syncserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Server — sync-сервер с мульти-пользовательской поддержкой.
type Server struct {
	storage Storage
}

// New создаёт сервер с SQL-хранилищем.
// driverName: "sqlite" или "pgx" (postgres).
// dsn: путь к файлу для SQLite, или строка подключения для Postgres.
func New(driverName, dsn string) (*Server, error) {
	dialect := "sqlite"
	if strings.Contains(driverName, "postgres") || driverName == "pgx" {
		dialect = "postgres"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	storage, err := NewSQLStorage(db, dialect)
	if err != nil {
		return nil, err
	}

	return &Server{storage: storage}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/register", s.handleRegister)
	mux.HandleFunc("POST /v1/login", s.handleLogin)
	mux.HandleFunc("DELETE /v1/logout", s.authenticated(s.handleLogout))
	mux.HandleFunc("GET /v1/snapshot", s.authenticated(s.handleSnapshot))
	mux.HandleFunc("POST /v1/push", s.authenticated(s.handlePush))
	return mux
}

// ── Auth middleware ───────────────────────────────────────────────────────────

type contextKey int

const ctxUser contextKey = 0

func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := s.storage.GetUserByToken(r.Context(), token)
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUser, user)
		next(w, r.WithContext(ctx))
	}
}

func userFrom(r *http.Request) User { return r.Context().Value(ctxUser).(User) }

// ── Handlers ─────────────────────────────────────────────────────────────────

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := randomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.storage.CreateUser(r.Context(), id, req.Username, string(hash)); err != nil {
		if errors.Is(err, ErrUserExists) {
			http.Error(w, "username already taken", http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "username": req.Username})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, err := s.storage.GetUser(r.Context(), req.Username)
	if errors.Is(err, ErrUserNotFound) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.storage.CreateSession(r.Context(), token, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.storage.DeleteSession(r.Context(), token)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	snap, err := s.storage.GetSnapshot(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if snap.Version == 0 && len(snap.Blob) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)

	var req struct {
		BaseVersion int    `json:"base_version"`
		Blob        []byte `json:"blob"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	snap, err := s.storage.PushSnapshot(r.Context(), user.ID, req.BaseVersion, req.Blob)
	if errors.Is(err, ErrVersionConflict) {
		http.Error(w, "version conflict", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
