// migrate.go applies the SQL in db/migrations to a database that has none of
// it yet. It is the deployed path only.
//
// Locally the schema arrives a different way: docker-compose.yml mounts
// db/migrations at /docker-entrypoint-initdb.d and Postgres applies it itself,
// once, on an empty volume. A database seeded that way has the schema and no
// schema_migrations rows, so running Migrate against it fails on "type source
// already exists". That is correct and deliberate: the two paths are not meant
// to be mixed, and a runner that swallowed the conflict would hide a genuinely
// divergent schema.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ledgerDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Migrate applies every *.sql file in src in filename order, recording each in
// schema_migrations so a second run is a no-op.
//
// It takes an fs.FS rather than embedding the SQL because //go:embed cannot
// reach outside its own package directory and db/ stays pure SQL. Callers pass
// os.DirFS("db/migrations").
func Migrate(ctx context.Context, src fs.FS) error {
	p, err := Pool(ctx)
	if err != nil {
		return err
	}

	files, err := fs.Glob(src, "*.sql")
	if err != nil {
		return fmt.Errorf("db: list migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("db: no *.sql found; check the fs.FS root")
	}
	// Filename order is migration order, which is why they are numbered.
	sort.Strings(files)

	if _, err := p.Exec(ctx, ledgerDDL); err != nil {
		return fmt.Errorf("db: create schema_migrations: %w", err)
	}

	applied, err := appliedFilenames(ctx, p)
	if err != nil {
		return err
	}

	for _, name := range files {
		if applied[name] {
			continue
		}
		if err := apply(ctx, p, src, name); err != nil {
			return err
		}
	}
	return nil
}

func appliedFilenames(ctx context.Context, p *pgxpool.Pool) (map[string]bool, error) {
	rows, err := p.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("db: read schema_migrations: %w", err)
	}

	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("db: read schema_migrations: %w", err)
	}

	applied := make(map[string]bool, len(names))
	for _, n := range names {
		applied[n] = true
	}
	return applied, nil
}

// apply runs one migration file and records it.
//
// The file is executed as a single Exec with no arguments, which pgx sends
// over the simple protocol, so a file containing many statements works. The
// runner opens no transaction of its own: a migration file owns its own, and
// 001_init.sql opens one explicitly. Nesting would mean the file's COMMIT
// closed the runner's transaction and left the ledger insert dangling outside
// it.
//
// The cost is a window: a crash between the COMMIT and the insert leaves a
// migration applied and unrecorded, and the next run fails on the conflict.
// That is a loud, manual failure rather than a half-applied schema, which is
// the right way round.
func apply(ctx context.Context, p *pgxpool.Pool, src fs.FS, name string) error {
	statements, err := fs.ReadFile(src, name)
	if err != nil {
		return fmt.Errorf("db: read %s: %w", name, err)
	}

	if _, err := p.Exec(ctx, string(statements)); err != nil {
		return fmt.Errorf("db: apply %s: %w", name, err)
	}

	if _, err := p.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("db: %s applied but not recorded, record it by hand before re-running: %w", name, err)
	}
	return nil
}
