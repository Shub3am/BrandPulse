// cache.go is the fetch cache, which is the cheapest credit we have: a hit
// costs nothing and a miss costs between one and twenty.
//
// It has two layers. The fetch_cache table is the one that survives a process,
// and it is keyed (source, query_hash, time_bucket) exactly as the schema is.
// In front of it sits a per-client map, because a fan-out repeats the same
// query inside one run and because a replay client has no pool to read at all.
//
// It must not widen a time bucket on its own. The bucket comes in with the
// request, and widening it is a decision about freshness that belongs to the
// method that knows what it fetched.
package anakin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"brandpulse/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cacheKey(source models.Source, queryHash, timeBucket string) string {
	return string(source) + "\x00" + queryHash + "\x00" + timeBucket
}

func (c *httpClient) recall(key string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, ok := c.memo[key]
	return payload, ok
}

func (c *httpClient) remember(key string, payload json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memo[key] = payload
}

// readFetchCache returns the cached payload for this key, and false when there
// is none.
func readFetchCache(ctx context.Context, pool *pgxpool.Pool, source models.Source, queryHash, timeBucket string) (json.RawMessage, bool, error) {
	var payload []byte

	// $1::source because the column is the Postgres enum and a parameter
	// arrives as text.
	err := pool.QueryRow(ctx,
		`SELECT payload FROM fetch_cache
		 WHERE source = $1::source AND query_hash = $2 AND time_bucket = $3`,
		string(source), queryHash, timeBucket).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("anakin: read the fetch cache for %s/%s: %w", source, timeBucket, err)
	}
	return payload, true, nil
}

// writeFetchCache stores a fresh payload. A concurrent collector may have
// written the same row a moment earlier, which is a race we win by ignoring:
// both callers fetched the same bytes for the same key.
func writeFetchCache(ctx context.Context, pool *pgxpool.Pool, source models.Source, queryHash, timeBucket string, payload json.RawMessage, credits int) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO fetch_cache (source, query_hash, time_bucket, payload, credits)
		 VALUES ($1::source, $2, $3, $4, $5)
		 ON CONFLICT (source, query_hash, time_bucket) DO NOTHING`,
		string(source), queryHash, timeBucket, []byte(payload), credits)
	if err != nil {
		return fmt.Errorf("anakin: write the fetch cache for %s/%s: %w", source, timeBucket, err)
	}
	return nil
}
