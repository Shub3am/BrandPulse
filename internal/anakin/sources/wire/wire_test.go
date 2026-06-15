package wire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordedDirs are the two sources the 2026-09-20 session recorded through
// Wire. Every file in them is a real poll response, so they are the only proof
// that matters here: the envelope this package strips is the one Anakin sent,
// not the one the research file describes.
var recordedDirs = []string{"../../../../fixtures/reddit", "../../../../fixtures/youtube"}

// liveEnvelope is one rt_search poll response, copied out of
// fixtures/reddit/2df29881…json and cut down to the keys Payload reads.
const liveEnvelope = `{
  "credits_used": 2,
  "data": {
    "status": "ok",
    "data": {"query": "suncoast aqua gel", "posts": [], "post_count": 0},
    "files": [],
    "error": null,
    "meta": {"action_id": "rt_search", "catalog_slug": "reddit", "envelope_version": "v1"}
  },
  "execution_ms": 3723,
  "status": "completed"
}`

func TestPayloadUnwrapsEveryRecordedResponse(t *testing.T) {
	for _, dir := range recordedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v; the recorded corpus is what these tests are for", dir, err)
		}
		if len(entries) == 0 {
			t.Fatalf("%s holds no recordings", dir)
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			t.Run(entry.Name(), func(t *testing.T) {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("reading %s: %v", path, err)
				}

				payload, err := Payload(raw)
				if err != nil {
					t.Fatalf("Payload: %v", err)
				}
				// The payload is an object keyed per action. An adapter that
				// received the envelope instead would unmarshal it into a
				// struct of zero values without an error, which is the whole
				// bug.
				var object map[string]json.RawMessage
				if err := json.Unmarshal(payload, &object); err != nil {
					t.Fatalf("the unwrapped payload is not an object: %v", err)
				}
				if _, wrapped := object["meta"]; wrapped {
					t.Errorf("the payload still carries the envelope's meta key: %s", truncate(payload))
				}
			})
		}
	}
}

func TestPayloadReturnsTheInnerData(t *testing.T) {
	payload, err := Payload(json.RawMessage(liveEnvelope))
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}

	var inner struct {
		Query     string            `json:"query"`
		Posts     []json.RawMessage `json:"posts"`
		PostCount int               `json:"post_count"`
	}
	if err := json.Unmarshal(payload, &inner); err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	if inner.Query != "suncoast aqua gel" {
		t.Errorf("query = %q, want the action's own payload two data hops down", inner.Query)
	}
}

func TestPayloadRefusesAnEnvelopeWithNoData(t *testing.T) {
	cases := map[string]string{
		"an empty inner envelope": `{"status":"completed","data":{"status":"ok","error":null}}`,
		"no outer data":           `{"status":"completed","credits_used":2}`,
		"a payload error":         `{"status":"completed","data":{"status":"error","error":"rate limited","data":null}}`,
		"not an envelope at all":  `{"posts":[],"post_count":0}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			payload, err := Payload(json.RawMessage(body))
			if err == nil {
				t.Fatalf("Payload returned %s, want an error rather than an empty result", payload)
			}
		})
	}
}

// A payload the adapter cannot read has to name the job status, or a silent
// zero is indistinguishable from a quiet week.
func TestPayloadNamesTheStatusItSaw(t *testing.T) {
	_, err := Payload(json.RawMessage(`{"status":"completed","data":{"status":"ok","error":null}}`))
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "completed") {
		t.Errorf("err = %v, want the job status in the message", err)
	}
}

func truncate(payload json.RawMessage) string {
	const limit = 200
	if len(payload) <= limit {
		return string(payload)
	}
	return string(payload[:limit]) + "..."
}
