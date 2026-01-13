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
// # DATABASE_URL carries the password
//
// No error in this package includes the URL. A connection failure is the most
// likely thing to be pasted into a chat window, and pgx does not redact it for
// you.
package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	once     sync.Once
	pool     *pgxpool.Pool
	poolErr  error
	poolOpen bool
)

// Pool returns the process-wide pool, built lazily from DATABASE_URL on first
// call. Every later call returns the same pool.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	once.Do(func() {
		pool, poolErr = open(ctx)
		poolOpen = poolErr == nil
	})
	return pool, poolErr
}

// Close releases the pool. Safe to call when no pool was ever built.
//
// This is process shutdown, not a reset: a Pool call after Close returns the
// closed pool, because the sync.Once has already fired. Agents call it from a
// defer in main and nothing else calls it.
func Close() {
	if poolOpen {
		pool.Close()
		poolOpen = false
	}
}

func open(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.New("db: DATABASE_URL is unset")
	}

	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: bad DATABASE_URL: %w", err)
	}

	// pgxpool.New does not connect, so without this a wrong host or password
	// surfaces at the first query in the middle of a run rather than at
	// startup.
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("db: cannot reach Postgres: %w", err)
	}
	return p, nil
}
