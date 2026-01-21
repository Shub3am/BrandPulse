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
// This package must not be used to build a dedupe key. Two ids are never
// equal, which is the opposite of what dedupe needs: that is hashing.ContentHash
// for mentions and Alert.DedupeKey for alerts.
package ids

import "github.com/oklog/ulid/v2"

// New returns a fresh id for prefix, formatted "<prefix>_<ulid>".
//
// ulid.Make is monotonic within a millisecond and safe for concurrent use, so
// a collector minting ten thousand mention ids in one tight loop still gets
// ids that sort in creation order.
func New(prefix string) string {
	return prefix + "_" + ulid.Make().String()
}
