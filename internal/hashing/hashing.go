// Package hashing produces the content hash that dedupes mentions.
//
// This package belongs to B1 (their Task 2). It is implemented here so track
// B4's crisis injection can populate Mention.ContentHash, which is NOT NULL and
// backs mentions UNIQUE (brand_id, content_hash). Replace it with B1's version
// when that lands.
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
	urlPattern    = regexp.MustCompile(`https?://\S+`)
	handlePattern = regexp.MustCompile(`@\w+`)
	spacePattern  = regexp.MustCompile(`\s+`)
)

// ContentHash is sha256 over normalised text plus the source, hex-encoded.
//
// Normalisation strips URLs and @handles and collapses whitespace, so the same
// post found by two different keywords, or reposted with a different tracking
// link, hashes equal. The source is mixed in because the same sentence on
// Reddit and on Amazon is two mentions, not one.
func ContentHash(text string, source models.Source) string {
	normalised := strings.ToLower(text)
	normalised = urlPattern.ReplaceAllString(normalised, " ")
	normalised = handlePattern.ReplaceAllString(normalised, " ")
	normalised = strings.TrimSpace(spacePattern.ReplaceAllString(normalised, " "))

	sum := sha256.Sum256([]byte(normalised + "|" + string(source)))
	return hex.EncodeToString(sum[:])
}
