// Offline tests for the jsonb helper in store.go. Separate from store_test.go
// because that file is skipped without BP_LIVE_DB and this needs no database:
// the bug it covers is a marshalling bug, and it should fail in CI.

package main

import "testing"

// TestJSONBNeverWritesNullIntoAnArrayColumn pins the shape a nil slice and a
// nil map take on the wire. A nil []string boxed into an any is a non-nil
// interface, so the v == nil guard misses it and json.Marshal writes null,
// which the NOT NULL on every one of these columns happily accepts because it
// is checking for a SQL NULL. The dashboard then calls .join on it and dies.
func TestJSONBNeverWritesNullIntoAnArrayColumn(t *testing.T) {
	var nilStrings []string
	var nilFloats map[string]float64
	var nilAny map[string]any

	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"nil slice", nilStrings, "[]"},
		{"empty slice", []string{}, "[]"},
		{"populated slice", []string{"refund"}, `["refund"]`},
		{"nil map", nilFloats, "{}"},
		{"nil any map", nilAny, "{}"},
		{"populated map", map[string]float64{"pos": 1}, `{"pos":1}`},
		{"untyped nil", nil, "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jsonb(tc.value)
			if err != nil {
				t.Fatalf("jsonb(%v): %v", tc.value, err)
			}
			if string(got) != tc.want {
				t.Errorf("jsonb(%v) = %s, want %s", tc.value, got, tc.want)
			}
		})
	}
}
