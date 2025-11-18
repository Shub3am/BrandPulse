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
// STUB: signatures only, bodies panic. B1 Task 2 implements this.
package hashing

import "brandpulse/internal/models"

// ContentHash returns the sha256 hex of text normalised for comparison:
// lowercased, whitespace runs collapsed, URLs and @handles stripped, with
// source mixed in so the same words on two platforms stay two mentions.
func ContentHash(text string, source models.Source) string {
	panic("not implemented")
}
