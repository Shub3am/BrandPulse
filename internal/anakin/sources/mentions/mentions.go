// Package mentions turns an adapter's parsed drafts into mentions the rest of
// BrandPulse will accept.
//
// It exists because the same closing steps end every adapter: drop what falls
// outside the collection window, drop what a negative keyword disqualifies,
// stamp the brand and an id, compute the dedupe hash, validate, and count what
// was thrown away. Six copies of that is six chances to forget one of them,
// and Mention.Lang has no default, so forgetting is a runtime rejection at the
// insert rather than a compile error.
//
// It must not know how any source formats a date, a rating or an author. A
// draft arrives with PostedAt already normalised to UTC and Lang already set;
// if that is wrong the adapter is wrong, and nothing here papers over it.
package mentions

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"brandpulse/internal/hashing"
	"brandpulse/internal/ids"
	"brandpulse/internal/models"
)

// Dropped counts what Filter and Finalize threw away, split by reason. The
// collector reports these: a source that returns nothing because every result
// fell outside the window is a different problem from one that returns nothing
// because the query was wrong, and a single "0 mentions" cannot tell them
// apart.
type Dropped struct {
	OutsideWindow   int
	NegativeKeyword int

	// Invalid holds one Validate error per rejected mention. These are bugs,
	// not filtering, so they carry their message rather than just a count.
	Invalid []string
}

// Total reports how many drafts were dropped for any reason.
func (d Dropped) Total() int {
	return d.OutsideWindow + d.NegativeKeyword + len(d.Invalid)
}

// Filter returns the drafts that fall inside [start, end) and carry no term
// from p.NegativeKeywords, with counts of what it removed.
//
// Negative-keyword filtering happens here, before enrichment, because a
// mention about Trent Boult costs the same LLM tokens as one about boult
// earbuds and is worth none of them.
func Filter(drafts []models.Mention, p models.BrandProfile, start, end time.Time) ([]models.Mention, Dropped) {
	negative := negativePattern(p.NegativeKeywords)

	var kept []models.Mention
	var dropped Dropped
	for _, d := range drafts {
		if d.PostedAt.Before(start) || !d.PostedAt.Before(end) {
			dropped.OutsideWindow++
			continue
		}
		if negative != nil && negative.MatchString(d.Text) {
			dropped.NegativeKeyword++
			continue
		}
		kept = append(kept, d)
	}
	return kept, dropped
}

// Stamp gives each mention its identity and rejects the ones that still fail
// Validate. A rejected mention is dropped and its error recorded: a batch with
// one invalid member must not fail the whole collection, per CONTRACTS §1.
func Stamp(kept []models.Mention, brandID string) ([]models.Mention, []string) {
	out := make([]models.Mention, 0, len(kept))
	var invalid []string
	for _, m := range kept {
		m.BrandID = brandID
		m.ID = ids.New("mnt")
		m.ContentHash = hashing.ContentHash(m.Text, m.Source)
		if err := m.Validate(); err != nil {
			invalid = append(invalid, fmt.Sprintf("%s %s: %v", m.Source, m.ExternalID, err))
			continue
		}
		out = append(out, m)
	}
	return out, invalid
}

// Finalize is Filter followed by Stamp, which is how every adapter closes.
func Finalize(drafts []models.Mention, p models.BrandProfile, start, end time.Time) ([]models.Mention, Dropped) {
	kept, dropped := Filter(drafts, p, start, end)
	out, invalid := Stamp(kept, p.BrandID)
	dropped.Invalid = invalid
	return out, dropped
}

// negativePattern compiles p.NegativeKeywords into one case-insensitive
// alternation, or nil when there is nothing to match.
//
// Word boundaries are not decoration. "ODI" is a negative keyword for the boAt
// profile and a plain substring match on "odi" also drops "melodic", which is
// a word that belongs in an audio-brand corpus.
func negativePattern(terms []string) *regexp.Regexp {
	var parts []string
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		quoted := regexp.QuoteMeta(term)
		// \b only means "boundary" next to a word character. Anchoring it
		// against a leading "#" or "@" would match nothing at all.
		if isWordByte(term[0]) {
			quoted = `\b` + quoted
		}
		if isWordByte(term[len(term)-1]) {
			quoted = quoted + `\b`
		}
		parts = append(parts, quoted)
	}
	if len(parts) == 0 {
		return nil
	}
	return regexp.MustCompile(`(?i)` + strings.Join(parts, "|"))
}

// isWordByte reports whether b is one of the ASCII characters Go's regexp \b
// treats as a word character.
func isWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}
