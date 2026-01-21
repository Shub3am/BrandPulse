// replay.go is the fixture half of the client: the file a replay reads and the
// file a record writes.
//
// A missing fixture is loud on purpose. Returning an empty result instead would
// let a collector report zero mentions for a source that was never recorded,
// and nobody finds that out until the dashboard is on a projector.
//
// It must not invent a payload. There is no fallback, no empty default and no
// "close enough" fixture: the path is named in the error and someone records
// it.
package anakin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"brandpulse/internal/models"
)

// fixturePath is fixtures/<source>/<query_hash>.json. The time bucket is
// deliberately not in it: a fixture is a recording of one query, and pinning it
// to the hour it was recorded would make every replay fail the next hour.
func fixturePath(dir string, source models.Source, queryHash string) string {
	return filepath.Join(dir, string(source), queryHash+".json")
}

func readFixture(dir string, source models.Source, queryHash string) (json.RawMessage, error) {
	path := fixturePath(dir, source, queryHash)

	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("anakin: no fixture at %s; record it with BP_FIXTURE_MODE=record, or check that the query matches the one that was recorded", path)
	}
	if err != nil {
		return nil, fmt.Errorf("anakin: read the fixture %s: %w", path, err)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("anakin: the fixture %s is not JSON", path)
	}
	return raw, nil
}

func writeFixture(dir string, source models.Source, queryHash string, payload json.RawMessage) error {
	path := fixturePath(dir, source, queryHash)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("anakin: make the fixture directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("anakin: write the fixture %s: %w", path, err)
	}
	return nil
}
