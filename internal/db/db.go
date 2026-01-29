// Package db owns the one Postgres connection pool and the migration runner.
//
// It deliberately does not wrap Query, QueryRow and Exec. pgxpool is already
// the API we want, and a passthrough layer is one more place a context gets
// dropped. Callers take the pool and use it directly, with pgx.CollectRows for
// slices.
//
// DATABASE_URL is the only configuration. Locally it points at the shared
// compose Postgres on host port 5433, not 5432, because another project
// commonly holds 5432 and compose then refuses to start at all.
//
// STUB: signatures only, bodies panic. B1 Task 4 implements this.
package db

import (
	"context"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool returns the process-wide pool, built lazily from DATABASE_URL on first
// call. Every later call returns the same pool.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	panic("not implemented")
}

// Close releases the pool. Safe to call when no pool was ever built.
func Close() {
	panic("not implemented")
}

// Migrate applies every *.sql file in src in filename order, recording each in
// schema_migrations so a second run is a no-op.
//
// It takes an fs.FS rather than embedding the SQL because //go:embed cannot
// reach outside its own package directory and db/ stays pure SQL. Callers pass
// os.DirFS("db/migrations").
//
// Nothing runs this at agent boot. Locally the compose mount at
// /docker-entrypoint-initdb.d applies the schema on an empty volume; this is
// the deployed path, where there is no compose.
func Migrate(ctx context.Context, src fs.FS) error {
	panic("not implemented")
}
