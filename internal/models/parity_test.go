// parity_test.go is the schema guard: it fails the build when a struct field
// and a Postgres column drift apart.
//
// It replaces the deleted shared/tests/test_contract_schema_parity.py. Go
// reflection stands in for pydantic introspection; the job is the same.
//
// This file must never open a database, a socket or an agent package. It reads
// one .sql file off disk and reflects over structs, so it runs in CI with no
// container, no key and no credit.
package models

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

const schemaPath = "../../db/migrations/001_init.sql"

// createTableRe pulls one table name and its raw body out of the migration.
// (?s) so . spans the body, and the lazy (.*?) stops at the first "\n);" so
// consecutive tables do not merge into one match.
var createTableRe = regexp.MustCompile(`(?s)CREATE TABLE (\w+) \((.*?)\n\);`)

// constraintOpeners are the words that begin a table-level constraint rather
// than a column. A line starting with one of these has no column name to take.
var constraintOpeners = []string{"UNIQUE", "PRIMARY", "FOREIGN", "CHECK", "CONSTRAINT"}

// parityCase pairs one wire struct with its storage table and the divergences
// docs/CONTRACTS.md §3b declares intentional. Anything outside those two sets
// is drift and fails.
type parityCase struct {
	model     any
	table     string
	modelOnly []string // on the wire, no column
	sqlOnly   []string // a column, never on the wire
}

var parityCases = map[string]parityCase{
	"BrandProfile": {
		model:     BrandProfile{},
		table:     "brand_profiles",
		modelOnly: []string{"name", "website"}, // denormalised from brands
		sqlOnly:   []string{"confirmed_at", "created_at"},
	},
	"Mention": {
		model:   Mention{},
		table:   "mentions",
		sqlOnly: []string{"collected_at"},
	},
	"Enrichment": {
		model:   Enrichment{},
		table:   "mention_enrichment",
		sqlOnly: []string{"created_at"},
	},
	"Topic": {
		model:     Topic{},
		table:     "topics",
		modelOnly: []string{"mention_ids", "top_examples"}, // topic_mentions join
		sqlOnly:   []string{"created_at"},
	},
	"Alert": {
		model:     Alert{},
		table:     "alerts",
		modelOnly: []string{"sample_mentions"},
		sqlOnly:   []string{"sample_mention_ids"},
		// created_at is deliberately in neither set. Alert carries CreatedAt on
		// the wire, so the column and the field are a matched pair.
	},
	"ReplyDraft": {
		model:   ReplyDraft{},
		table:   "reply_drafts",
		sqlOnly: []string{"created_at"},
		// modelOnly is empty where the Python test had requires_human_approval.
		// ReplyDraft has no such field, so reflection never sees it. The
		// invariant is proved by TestAReplyDraftAlwaysRequiresHumanApproval.
	},
	"RunRecord": {
		model: RunRecord{},
		table: "runs",
	},
}

func TestSchemaParity(t *testing.T) {
	tables := parseSchema(t)

	for name, tc := range parityCases {
		t.Run(name, func(t *testing.T) {
			columns, ok := tables[tc.table]
			if !ok {
				t.Fatalf("%s has no table %q in %s", name, tc.table, schemaPath)
			}
			fields := wireFields(tc.model)

			if extra := missing(fields, columns, tc.modelOnly); len(extra) > 0 {
				t.Errorf("%s has json tags with no column in %s: %v\n"+
					"Either add the column in a migration or add the field to §3b and to this case.",
					name, tc.table, extra)
			}
			if extra := missing(columns, fields, tc.sqlOnly); len(extra) > 0 {
				t.Errorf("table %s has columns with no json tag on %s: %v\n"+
					"Either add the field or declare the column storage-only in §3b and in this case.",
					tc.table, name, extra)
			}
		})
	}
}

// TestAReplyDraftAlwaysRequiresHumanApproval is the Go port of the Python test
// of the same name. The field does not exist, so this asserts on the emitted
// JSON rather than on the struct: MarshalJSON writes the key unconditionally
// and no payload can turn it off.
func TestAReplyDraftAlwaysRequiresHumanApproval(t *testing.T) {
	raw, err := json.Marshal(NewReplyDraft("drf_1", ChannelEmail))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}

	got, present := wire["requires_human_approval"]
	if !present {
		t.Fatalf("requires_human_approval is absent from %s", raw)
	}
	if got != true {
		t.Errorf("requires_human_approval = %v, want true", got)
	}
}

// TestIdempotencyKeysAreRequired ports the Python test behaviourally. Go has no
// required-field metadata, so the guard is Validate rejecting the empty value
// rather than a decoder refusing to build the struct.
func TestIdempotencyKeysAreRequired(t *testing.T) {
	t.Run("Alert without a dedupe key", func(t *testing.T) {
		a := NewAlert("alr_1", "brd_1", AlertSpike, SeverityHigh, "", time.Now())
		a.Evidence = []AlertEvidence{{}}
		if err := a.Validate(); err == nil {
			t.Fatal("Validate accepted an empty DedupeKey; the insert would fail on alerts UNIQUE (brand_id, dedupe_key)")
		}
	})

	t.Run("RunRecord without a time bucket", func(t *testing.T) {
		r := NewRunRecord("run_1", "brd_1", RunScheduled, "", time.Now())
		if err := r.Validate(); err == nil {
			t.Fatal("Validate accepted an empty TimeBucket; the insert would fail on runs UNIQUE (brand_id, kind, time_bucket)")
		}
	})
}

// parseSchema returns table name -> set of column names.
func parseSchema(t *testing.T) map[string]map[string]bool {
	t.Helper()

	sql, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	matches := createTableRe.FindAllStringSubmatch(string(sql), -1)
	if len(matches) == 0 {
		t.Fatalf("no CREATE TABLE found in %s; the regexp and the file disagree", schemaPath)
	}

	tables := make(map[string]map[string]bool, len(matches))
	for _, m := range matches {
		tables[m[1]] = columnNames(m[2])
	}
	return tables
}

// columnNames takes the first token of every line in a CREATE TABLE body that
// declares a column.
func columnNames(body string) map[string]bool {
	columns := make(map[string]bool)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		name, _, _ := strings.Cut(line, " ")
		if isConstraint(name) {
			continue
		}
		columns[name] = true
	}
	return columns
}

func isConstraint(firstToken string) bool {
	for _, opener := range constraintOpeners {
		if strings.EqualFold(firstToken, opener) {
			return true
		}
	}
	return false
}

// wireFields returns the json tag names of v's exported fields, which is the
// wire format by definition.
//
// A field with no json tag is reported under its Go name so it surfaces as
// drift rather than passing silently. Every field in this package is tagged and
// that is the point: an untagged one would marshal as "SomeField".
func wireFields(v any) map[string]bool {
	rt := reflect.TypeOf(v)
	fields := make(map[string]bool, rt.NumField())

	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = true
	}
	return fields
}

// missing returns the members of have that are absent from want and not
// declared as an intended divergence, sorted so the failure message is stable.
func missing(have, want map[string]bool, allowed []string) []string {
	exempt := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		exempt[a] = true
	}

	var out []string
	for name := range have {
		if !want[name] && !exempt[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
