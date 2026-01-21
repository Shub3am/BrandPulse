// Package hashing produces the content hash that dedupes mentions.
//
// The same Reddit thread found by two different keywords, or the same tweet
// carrying a different tracking link, must collapse to one row. That is what
// mentions.UNIQUE (brand_id, content_hash) enforces and what this package
// computes.
//
// Importing models is fine here: models imports nothing internal, so there is
// no cycle.
//
// # This hash is not stable across versions
//
// Changing the normalisation changes every hash, which means the next run
// re-inserts mentions it already had. If a rule below has to change, it is a
// migration that recomputes the column, not a one-line edit.
package hashing

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"brandpulse/internal/models"
)

// Compiled once: this runs on every mention text in every run.
var (
	// urlPattern covers the two forms that actually appear in mention text, a
	// scheme-qualified link and a bare www host. Tracking parameters live
	// inside the match, so they go with it.
	urlPattern = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)

	// handlePattern is an @mention. Stripped because the same complaint
	// reposted with the brand tagged is the same complaint.
	handlePattern = regexp.MustCompile(`@\w+`)

	whitespaceRun = regexp.MustCompile(`\s+`)
)

// ContentHash returns the sha256 hex of text normalised for comparison:
// lowercased, whitespace runs collapsed, URLs and @handles stripped, with
// source mixed in so the same words on two platforms stay two mentions.
func ContentHash(text string, source models.Source) string {
	// The separator is a byte that cannot occur in a Source value, so a source
	// ending in the first characters of a text cannot forge another pair's
	// input.
	sum := sha256.Sum256([]byte(string(source) + "\x00" + normalise(text)))
	return hex.EncodeToString(sum[:])
}

// normalise strips everything that varies between two copies of the same post.
func normalise(text string) string {
	text = strings.ToLower(text)
	text = urlPattern.ReplaceAllString(text, " ")
	text = handlePattern.ReplaceAllString(text, " ")
	text = whitespaceRun.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
