// db_test.go runs against the shared compose Postgres, in a scratch database
// of its own.
//
// It creates "brandpulse_migrate_test_<pid>" and drops it again. It never
// touches the brandpulse database, because that volume is shared by all seven
// worktrees and this test has to be safe to run while five other tracks are
// working. A scratch database is also the only honest way to test Migrate: the
// brandpulse database gets its schema from the compose initdb mount and has no
// schema_migrations rows, so Migrate is expected to fail there.
package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// migrationsDir is db/migrations relative to this package.
const migrationsDir = "../../db/migrations"

// skipReason is empty when the scratch database is up. Set by TestMain.
var skipReason string

func TestMain(m *testing.M) {
	admin := os.Getenv("DATABASE_URL")
	if admin == "" {
		skipReason = "DATABASE_URL is unset; start the compose Postgres and export it"
		os.Exit(m.Run())
	}

	scratch, drop, err := createScratchDatabase(admin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db_test: %v\n", err)
		os.Exit(1)
	}

	// Pool reads this on its first call, which has not happened yet.
	os.Setenv("DATABASE_URL", scratch)

	code := m.Run()

	Close()
	drop()
	os.Exit(code)
}

// TestMigrateIsIdempotent is the whole point of schema_migrations: the
// deployed path runs Migrate on every boot and the second boot must be a
// no-op, not a pile of "already exists" errors.
func TestMigrateIsIdempotent(t *testing.T) {
	requireDatabase(t)

	ctx := context.Background()
	src := os.DirFS(migrationsDir)

	if err := Migrate(ctx, src); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(ctx, src); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	pool, err := Pool(ctx)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}

	var recorded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&recorded); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}

	onDisk, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read %s: %v", migrationsDir, err)
	}
	want := 0
	for _, e := range onDisk {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			want++
		}
	}

	if recorded != want {
		t.Errorf("schema_migrations has %d rows, want one per .sql file (%d)", recorded, want)
	}
}

// TestMigrateBuiltTheSchema checks that the ledger is not the only thing that
// happened. A runner that recorded filenames and applied nothing would pass
// the test above.
func TestMigrateBuiltTheSchema(t *testing.T) {
	requireDatabase(t)

	ctx := context.Background()
	if err := Migrate(ctx, os.DirFS(migrationsDir)); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	pool, err := Pool(ctx)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}

	for _, table := range []string{"brands", "mentions", "alerts", "runs", "fetch_cache"} {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s was not created", table)
		}
	}
}

func TestPoolReturnsTheSameInstance(t *testing.T) {
	requireDatabase(t)

	ctx := context.Background()
	first, err := Pool(ctx)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}
	second, err := Pool(ctx)
	if err != nil {
		t.Fatalf("second Pool: %v", err)
	}

	if first != second {
		t.Error("Pool built a second pool; the sync.Once is not doing its job")
	}
}

func requireDatabase(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip(skipReason)
	}
}

// createScratchDatabase makes an empty database on the same server and returns
// its URL plus a function that drops it.
func createScratchDatabase(adminURL string) (string, func(), error) {
	name := fmt.Sprintf("brandpulse_migrate_test_%d", os.Getpid())

	scratchURL, err := swapDatabase(adminURL, name)
	if err != nil {
		return "", nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", nil, fmt.Errorf("connect to create the scratch database: %w", err)
	}
	defer conn.Close(ctx)

	// Identifiers cannot be parameterised, and this one is built from a pid.
	if _, err := conn.Exec(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`); err != nil {
		return "", nil, fmt.Errorf("drop a leftover %s: %w", name, err)
	}
	if _, err := conn.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	drop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "db_test: leaked scratch database %s: %v\n", name, err)
			return
		}
		defer conn.Close(ctx)

		if _, err := conn.Exec(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`); err != nil {
			fmt.Fprintf(os.Stderr, "db_test: leaked scratch database %s: %v\n", name, err)
		}
	}
	return scratchURL, drop, nil
}

func swapDatabase(rawURL, name string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Not wrapped with the URL: it carries the password.
		return "", fmt.Errorf("DATABASE_URL is not a URL: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}
