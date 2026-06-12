package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yue2u/marina/internal/core"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct{ db *sql.DB }

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// --- Hosts ---

func (s *SQLiteStore) SaveHost(ctx context.Context, h core.Host) error {
	optionsJSON, err := json.Marshal(h.Options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}
	tagsJSON, err := json.Marshal(h.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	var deletedAt sql.NullString
	if h.DeletedAt != nil {
		deletedAt = sql.NullString{String: h.DeletedAt.Format(time.RFC3339), Valid: true}
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO hosts (id, folder_id, label, hostname, port, username, auth,
              identity_id, secret_ref, jump_host_id, options, tags, updated_at, deleted_at)
          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
          ON CONFLICT(id) DO UPDATE SET
              label=excluded.label, hostname=excluded.hostname, port=excluded.port,
              username=excluded.username, auth=excluded.auth, identity_id=excluded.identity_id,
              secret_ref=excluded.secret_ref, jump_host_id=excluded.jump_host_id,
              options=excluded.options, tags=excluded.tags,
              updated_at=excluded.updated_at, deleted_at=excluded.deleted_at`,
		h.ID, h.FolderID, h.Label, h.Hostname, h.Port, h.Username, h.Auth,
		h.IdentityID, h.SecretRef, h.JumpHostID,
		string(optionsJSON), string(tagsJSON),
		h.UpdatedAt.Format(time.RFC3339), deletedAt,
	)
	return err
}

func (s *SQLiteStore) Hosts(ctx context.Context, folderID *string) ([]core.Host, error) {
	query := `SELECT id, folder_id, label, hostname, port, username, auth,
          identity_id, secret_ref, jump_host_id, options, tags, updated_at, deleted_at
          FROM hosts WHERE deleted_at IS NULL`
	args := []any{}
	if folderID != nil {
		query += " AND folder_id = ?"
		args = append(args, *folderID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []core.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *SQLiteStore) Host(ctx context.Context, id string) (core.Host, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, folder_id, label, hostname, port, username, auth,
          identity_id, secret_ref, jump_host_id, options, tags, updated_at, deleted_at
          FROM hosts WHERE id = ?`, id)
	if err != nil {
		return core.Host{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return core.Host{}, fmt.Errorf("host %q not found", id)
	}
	return scanHost(rows)
}

func (s *SQLiteStore) DeleteHost(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE hosts SET deleted_at = ? WHERE id = ?`,
		time.Now().Format(time.RFC3339), id)
	return err
}

func scanHost(rows *sql.Rows) (core.Host, error) {
	var h core.Host
	var folderID, identityID, jumpHostID, deletedAt sql.NullString
	var optionsJSON, tagsJSON, updatedAtStr string

	err := rows.Scan(
		&h.ID, &folderID, &h.Label, &h.Hostname, &h.Port, &h.Username, &h.Auth,
		&identityID, &h.SecretRef, &jumpHostID,
		&optionsJSON, &tagsJSON, &updatedAtStr, &deletedAt,
	)
	if err != nil {
		return h, err
	}

	if folderID.Valid {
		h.FolderID = &folderID.String
	}
	if identityID.Valid {
		h.IdentityID = &identityID.String
	}
	if jumpHostID.Valid {
		h.JumpHostID = &jumpHostID.String
	}

	if h.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return h, fmt.Errorf("parse updated_at: %w", err)
	}
	if deletedAt.Valid {
		t, err := time.Parse(time.RFC3339, deletedAt.String)
		if err != nil {
			return h, fmt.Errorf("parse deleted_at: %w", err)
		}
		h.DeletedAt = &t
	}

	if err := json.Unmarshal([]byte(optionsJSON), &h.Options); err != nil {
		return h, fmt.Errorf("unmarshal options: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &h.Tags); err != nil {
		return h, fmt.Errorf("unmarshal tags: %w", err)
	}

	return h, nil
}

// --- Folders ---

func (s *SQLiteStore) SaveFolder(ctx context.Context, f core.Folder) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO folders (id, parent_id, name, sort_order)
          VALUES (?, ?, ?, ?)
          ON CONFLICT(id) DO UPDATE SET
              parent_id=excluded.parent_id, name=excluded.name, sort_order=excluded.sort_order`,
		f.ID, f.ParentID, f.Name, f.Order)
	return err
}

func (s *SQLiteStore) Folders(ctx context.Context) ([]core.Folder, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, parent_id, name, sort_order FROM folders WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []core.Folder
	for rows.Next() {
		var f core.Folder
		var parentID sql.NullString
		if err := rows.Scan(&f.ID, &parentID, &f.Name, &f.Order); err != nil {
			return nil, err
		}
		if parentID.Valid {
			f.ParentID = &parentID.String
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (s *SQLiteStore) Folder(ctx context.Context, folderID string) (core.Folder, error) {
	var f core.Folder
	var parentID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, parent_id, name, sort_order FROM folders WHERE id = ? AND deleted_at IS NULL`,
		folderID).Scan(&f.ID, &parentID, &f.Name, &f.Order)
	if err == sql.ErrNoRows {
		return f, fmt.Errorf("folder %q not found", folderID)
	}
	if err != nil {
		return f, err
	}
	if parentID.Valid {
		f.ParentID = &parentID.String
	}
	return f, nil
}

func (s *SQLiteStore) DeleteFolder(ctx context.Context, folderID string) {
	s.db.ExecContext(ctx,
		`UPDATE folders SET deleted_at = ? WHERE id = ?`,
		time.Now().Format(time.RFC3339), folderID)
}

// --- Identities ---

func (s *SQLiteStore) SaveIdentity(ctx context.Context, i core.Identity) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO identities (id, name, auth, secret_ref, updated_at)
          VALUES (?, ?, ?, ?, ?)
          ON CONFLICT(id) DO UPDATE SET
              name=excluded.name, auth=excluded.auth,
              secret_ref=excluded.secret_ref, updated_at=excluded.updated_at`,
		i.ID, i.Name, i.Auth, i.SecretRef, i.UpdatedAt.Format(time.RFC3339))
	return err
}

func (s *SQLiteStore) Identities(ctx context.Context) ([]core.Identity, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, auth, secret_ref, updated_at FROM identities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []core.Identity
	for rows.Next() {
		var i core.Identity
		var updatedAtStr string
		if err := rows.Scan(&i.ID, &i.Name, &i.Auth, &i.SecretRef, &updatedAtStr); err != nil {
			return nil, err
		}
		if i.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}
		identities = append(identities, i)
	}
	return identities, rows.Err()
}

func (s *SQLiteStore) Identity(ctx context.Context, identityID string) (core.Identity, error) {
	var i core.Identity
	var updatedAtStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, auth, secret_ref, updated_at FROM identities WHERE id = ?`,
		identityID).Scan(&i.ID, &i.Name, &i.Auth, &i.SecretRef, &updatedAtStr)
	if err == sql.ErrNoRows {
		return i, fmt.Errorf("identity %q not found", identityID)
	}
	if err != nil {
		return i, err
	}
	var parseErr error
	if i.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr); parseErr != nil {
		return i, fmt.Errorf("parse updated_at: %w", parseErr)
	}
	return i, nil
}

func (s *SQLiteStore) DeleteIdentity(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM identities WHERE id = ?`, id)
	return err
}
