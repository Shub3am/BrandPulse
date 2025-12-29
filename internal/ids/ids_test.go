package ids

import (
	"strings"
	"sync"
	"testing"
)

func TestNewFormatsPrefixUnderscoreULID(t *testing.T) {
	for _, prefix := range []string{"brd", "mnt", "top", "alr", "drf", "brf", "run"} {
		t.Run(prefix, func(t *testing.T) {
			id := New(prefix)

			got, rest, found := strings.Cut(id, "_")
			if !found {
				t.Fatalf("New(%q) = %q, want one underscore separator", prefix, id)
			}
			if got != prefix {
				t.Errorf("prefix = %q, want %q", got, prefix)
			}
			// A ULID is 26 characters of Crockford base32.
			if len(rest) != 26 {
				t.Errorf("ulid part %q is %d chars, want 26", rest, len(rest))
			}
		})
	}
}

// TestNewSortsChronologically is the reason this package exists. A caller that
// orders by id is ordering by creation time, so ids minted in sequence must
// come out ascending even inside the same millisecond.
func TestNewSortsChronologically(t *testing.T) {
	const n = 1000

	previous := New("mnt")
	for i := 1; i < n; i++ {
		current := New("mnt")
		if current <= previous {
			t.Fatalf("id %d (%q) does not sort after %q", i, current, previous)
		}
		previous = current
	}
}

// TestNewIsUniqueUnderConcurrency covers the collector, which mints ids from
// several source adapters at once. Run with -race.
func TestNewIsUniqueUnderConcurrency(t *testing.T) {
	const goroutines, each = 8, 500

	minted := make(chan string, goroutines*each)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				minted <- New("mnt")
			}
		}()
	}
	wg.Wait()
	close(minted)

	seen := make(map[string]bool, goroutines*each)
	for id := range minted {
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != goroutines*each {
		t.Errorf("minted %d unique ids, want %d", len(seen), goroutines*each)
	}
}
