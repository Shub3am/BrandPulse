package hashing

import (
	"testing"

	"brandpulse/internal/models"
)

// TestSameContentHashesEqual is the reason the package exists: the same post
// reaching us twice, differing only in the noise normalisation strips, must
// collapse to one mentions row.
func TestSameContentHashesEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "different trailing tracking links",
			a:    "Ordered from @mamaearth and the pump broke on day two https://t.co/abc123",
			b:    "Ordered from @mamaearth and the pump broke on day two https://t.co/zzz999",
		},
		{
			name: "a link on one copy and none on the other",
			a:    "delivery was four days late again http://bit.ly/xyz",
			b:    "delivery was four days late again",
		},
		{
			name: "different casing",
			a:    "Delivery Was Four Days Late Again",
			b:    "delivery was four days late again",
		},
		{
			name: "collapsed whitespace and newlines",
			a:    "delivery was\n\nfour   days late\tagain",
			b:    "delivery was four days late again",
		},
		{
			name: "leading and trailing whitespace",
			a:    "   delivery was four days late again  ",
			b:    "delivery was four days late again",
		},
		{
			name: "the brand tagged in one copy only",
			a:    "@mamaearth delivery was four days late again",
			b:    "delivery was four days late again",
		},
		{
			name: "a bare www host",
			a:    "compare prices at www.example.com/deal?utm_source=x before buying",
			b:    "compare prices at before buying",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := ContentHash(tc.a, models.SourceX), ContentHash(tc.b, models.SourceX); got != want {
				t.Errorf("hashes differ\n a = %q -> %s\n b = %q -> %s", tc.a, got, tc.b, want)
			}
		})
	}
}

func TestDifferentContentHashesDifferently(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "different words",
			a:    "delivery was four days late again",
			b:    "delivery was two days late again",
		},
		{
			name: "negation is not noise",
			a:    "the pump broke",
			b:    "the pump did not break",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ContentHash(tc.a, models.SourceX) == ContentHash(tc.b, models.SourceX) {
				t.Errorf("%q and %q collapsed to one hash", tc.a, tc.b)
			}
		})
	}
}

// TestSourceIsPartOfTheHash pins the second half of the dedupe rule. The same
// sentence posted on X and on Reddit is two mentions, because share of voice
// counts per source.
func TestSourceIsPartOfTheHash(t *testing.T) {
	const text = "delivery was four days late again"

	if ContentHash(text, models.SourceX) == ContentHash(text, models.SourceReddit) {
		t.Error("x and reddit produced the same hash for the same text")
	}
}

// TestEmptyTextStillHashes covers the review with a star rating and no body.
// Those mentions exist and still need a hash to insert under.
func TestEmptyTextStillHashes(t *testing.T) {
	if got := ContentHash("", models.SourceAmazon); len(got) != 64 {
		t.Errorf("ContentHash(\"\", amazon) = %q, want 64 hex chars", got)
	}
}

func TestHashIsHexAndFixedWidth(t *testing.T) {
	got := ContentHash("delivery was four days late again", models.SourceX)

	if len(got) != 64 {
		t.Fatalf("hash %q is %d chars, want 64", got, len(got))
	}
	for i, r := range got {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'f') {
			t.Fatalf("hash %q has non-hex %q at %d", got, r, i)
		}
	}
}
