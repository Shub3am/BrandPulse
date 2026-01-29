// Package ids mints the prefixed identifiers every BrandPulse record carries.
//
// An id is "<prefix>_<ulid>". ULID rather than UUID because it sorts by
// creation time as a string, so "ORDER BY id" is chronological and a human
// reading a log can tell which of two ids came first.
//
// The prefixes in use: brd (brand), mnt (mention), top (topic), alr (alert),
// drf (reply draft), brf (brief), run (pipeline run). They are documented here
// rather than in a constant block because a constant nobody imports drifts
// from the call sites that actually spell the prefix.
//
// STUB: signatures only, bodies panic. B1 Task 2 implements this.
package ids

// New returns a fresh id for prefix, formatted "<prefix>_<ulid>".
func New(prefix string) string {
	panic("not implemented")
}
