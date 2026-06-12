package store

import (
	"context"

	"github.com/Yue2u/marina/internal/core"
)

type Store interface {
	Hosts(ctx context.Context, folderID *string) ([]core.Host, error)
	Host(ctx context.Context, id string) (core.Host, error)
	SaveHost(ctx context.Context, h core.Host) error
	DeleteHost(ctx context.Context, id string) error // soft-deletion

	Folders(ctx context.Context) ([]core.Folder, error)
	Folder(ctx context.Context, folderID string) (core.Folder, error)
	SaveFolder(ctx context.Context, f core.Folder) error
	DeleteFolder(ctx context.Context, folderID string) // soft-deletion

	Identities(ctx context.Context) ([]core.Identity, error)
	Identity(ctx context.Context, identityID string) (core.Identity, error)
	SaveIdentity(ctx context.Context, i core.Identity) error
	DeleteIdentity(ctx context.Context, identityID string) error

	Close() error
}
