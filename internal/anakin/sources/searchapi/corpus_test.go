// corpus_test.go maps every Search result recorded live on 2026-09-20.
//
// The stubbed tests in searchapi_test.go use three hand-picked results. This
// one uses all 120 that were paid for, because the reason the recording
// produced zero web and zero news mentions was a date field that is blank far
// more often than three samples suggested.
package searchapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"brandpulse/internal/models"
)

// recordedDir holds the Search responses. They are filed under the web source
// because Client.Search has no source of its own; news results are in here too.
const recordedDir = "../../../../fixtures/web"

func TestEveryRecordedResultBecomesADraft(t *testing.T) {
	entries, err := os.ReadDir(recordedDir)
	if err != nil {
		t.Fatalf("reading %s: %v", recordedDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s holds no recordings", recordedDir)
	}

	undatedAt := UndatedAt(windowEnd)
	var results, undated int
	for _, entry := range entries {
		path := filepath.Join(recordedDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		var resp response
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("decoding %s: %v", path, err)
		}

		for _, result := range resp.Results {
			results++
			draft, err := ToDraft(result, models.SourceWeb, "recorded", undatedAt)
			if err != nil {
				t.Errorf("%s: %s: %v", entry.Name(), result.URL, err)
				continue
			}
			if draft.PostedAt.IsZero() {
				t.Errorf("%s: %s has a zero PostedAt, which fails Mention.Validate", entry.Name(), result.URL)
			}
			if draft.Raw["posted_at_precision"] == "unknown" {
				undated++
				if !draft.PostedAt.Equal(undatedAt) {
					t.Errorf("%s: %s is dated %v, want the window end %v", entry.Name(), result.URL, draft.PostedAt, undatedAt)
				}
			}
		}
	}

	if results == 0 {
		t.Fatal("the recordings hold no results at all")
	}
	// Roughly half the corpus carries no date. Dropping it, which is what the
	// adapter used to do, is most of why a paid recording produced no mentions.
	if undated == 0 {
		t.Errorf("none of the %d recorded results is undated; the fallback is untested by this corpus", results)
	}
	t.Logf("%d recorded results, %d of them undated", results, undated)
}
