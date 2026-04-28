// Package db is the only path to Postgres in this repo.
//
// Callers use the pool directly. Query, QueryRow and Exec are deliberately not
// re-wrapped: pgxpool already has them and a passthrough layer is one more
// place a context gets dropped.
//
// This package belongs to B1 (their Task 4). Pool and Close are implemented
// here because track B4's demo commands have to reach a real database to be
// worth anything; Migrate stays a panicking stub, and the compose mount at
// /docker-entrypoint-initdb.d applies 001_init.sql on an empty volume anyway.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	once sync.Once
	pool *pgxpool.Pool
	err  error
)

// Pool returns the process-wide connection pool, built lazily from
// DATABASE_URL. The host port is 5433, not 5432: another project's Postgres
// commonly holds 5432 on a developer machine.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	once.Do(func() {
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			err = fmt.Errorf("db: DATABASE_URL is not set")
			return
		}
		pool, err = pgxpool.New(ctx, url)
	})
	return pool, err
}

// Close releases the pool. Safe to call when Pool was never reached.
func Close() {
	if pool != nil {
		pool.Close()
	}
}

// Migrate applies *.sql from src in filename order, tracked in
// schema_migrations so it is idempotent. Callers pass
// os.DirFS("db/migrations").
func Migrate(ctx context.Context, src fs.FS) error {
	panic("not implemented: B1 Task 4 owns db.Migrate")
}
