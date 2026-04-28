// Package ids mints the prefixed, lexicographically sortable identifiers every
// BrandPulse row uses.
//
// The prefixes in use: brd (brand), mnt (mention), top (topic), alr (alert),
// drf (reply draft), brf (brief), run (pipeline run).
//
// This package belongs to B1 (their Task 2, which specifies
// github.com/oklog/ulid/v2). It is implemented here without that dependency so
// track B4 can run its tests before B1 lands; the output format is identical, a
// 26-character Crockford base32 ULID, so swapping in B1's version changes no
// caller and no stored id.
package ids

import (
	"crypto/rand"
	"time"
)

// crockford is Crockford's base32 alphabet: no I, L, O or U, so an id read
// aloud off a dashboard cannot be transcribed wrong.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New returns "<prefix>_<ulid>": 48 bits of millisecond timestamp followed by
// 80 bits of randomness, base32-encoded. Sortable by creation time, which is
// why the timestamp leads.
func New(prefix string) string {
	var raw [16]byte

	ms := uint64(time.Now().UTC().UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	if _, err := rand.Read(raw[6:]); err != nil {
		// crypto/rand does not fail on any platform we ship to, and an id we
		// cannot trust to be unique would corrupt a unique constraint silently.
		panic("ids: crypto/rand failed: " + err.Error())
	}

	return prefix + "_" + encode(raw)
}

// encode writes the 128 bits as 26 base32 characters, most significant first.
// The first character carries only 2 bits of the 130-bit field, which is why a
// ULID's leading digit never exceeds 7.
func encode(raw [16]byte) string {
	out := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		bit := (25 - i) * 5
		out[i] = crockford[extract(raw, bit)]
	}
	return string(out)
}

// extract reads the 5-bit group starting bit positions from the low end.
func extract(raw [16]byte, bit int) byte {
	var v byte
	for n := 0; n < 5; n++ {
		p := bit + n
		if p >= 128 {
			continue
		}
		byteIdx := 15 - p/8
		if raw[byteIdx]&(1<<uint(p%8)) != 0 {
			v |= 1 << uint(n)
		}
	}
	return v
}
