package store

import "context"

const schema = `
CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY, parent_id TEXT, name TEXT NOT NULL, sort_order INT, deleted_at TEXT
);
CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  folder_id TEXT,
  label TEXT NOT NULL,
  hostname TEXT NOT NULL,
  port INT NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  auth TEXT NOT NULL,
  identity_id TEXT,
  secret_ref TEXT,
  jump_host_id TEXT,
  options TEXT,
  tags TEXT,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
CREATE TABLE IF NOT EXISTS identities (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  auth TEXT NOT NULL,
  secret_ref TEXT,
  updated_at TEXT NOT NULL
);
`

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}
