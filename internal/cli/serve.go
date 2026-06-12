package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Yue2u/marina/internal/syncserver"
)

func serveCmd() *cobra.Command {
	var addr, dataDir, dbDriver, dbDSN string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a sync server",
		Long: `Run a multi-user sync server.

Default: SQLite в --data/sync.sqlite.
Для PostgreSQL: --db-driver pgx --db-dsn "postgres://user:pass@host/db"
(требует: go get github.com/jackc/pgx/v5/stdlib)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := dbDSN
			if dsn == "" {
				if dbDriver != "sqlite" {
					return fmt.Errorf("--db-dsn required for driver %q", dbDriver)
				}
				if err := os.MkdirAll(dataDir, 0700); err != nil {
					return fmt.Errorf("create data dir: %w", err)
				}
				dsn = "file:" + filepath.Join(dataDir, "sync.sqlite") + "?_pragma=foreign_keys(1)"
			}

			srv, err := syncserver.New(dbDriver, dsn)
			if err != nil {
				return fmt.Errorf("init server: %w", err)
			}

			fmt.Printf("marina sync server listening on %s (driver: %s)\n", addr, dbDriver)
			return http.ListenAndServe(addr, srv.Handler())
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8443", "Listen address")
	cmd.Flags().StringVar(&dataDir, "data", "/var/lib/marina", "Data directory (SQLite only)")
	cmd.Flags().StringVar(&dbDriver, "db-driver", "sqlite", "Database driver: sqlite | pgx")
	cmd.Flags().StringVar(&dbDSN, "db-dsn", "", "Database DSN (optional; default: --data/sync.sqlite)")
	return cmd
}
