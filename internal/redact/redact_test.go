package redact

import (
	"strings"
	"testing"
)

func TestPIIRedacts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain email",
			in:   "write to me at priya.sharma@gmail.com please",
			want: "write to me at [email] please",
		},
		{
			name: "email with a plus tag",
			in:   "order under priya+shop@example.co.in",
			want: "order under [email]",
		},
		{
			name: "ten digit mobile",
			in:   "call me on 9876543210 today",
			want: "call me on [phone] today",
		},
		{
			name: "country code with spaces",
			in:   "reach me: +91 98765 43210",
			want: "reach me: [phone]",
		},
		{
			name: "country code with hyphens",
			in:   "reach me: +91-98765-43210",
			want: "reach me: [phone]",
		},
		{
			name: "country code run together",
			in:   "+919876543210 is my whatsapp",
			want: "[phone] is my whatsapp",
		},
		{
			name: "both in one sentence",
			in:   "priya@example.com or 9876543210",
			want: "[email] or [phone]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PII(tc.in); got != tc.want {
				t.Errorf("PII(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestPIILeavesTheDataAlone is the other half of the contract. Handles and
// links are public, the responder needs them, and a redacted URL makes a reply
// draft nonsense.
func TestPIILeavesTheDataAlone(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"a handle", "@mamaearth this pump broke on day two"},
		{"a handle that looks like an address", "ping @mamaearth about it"},
		{"an https link", "photo here https://t.co/abc123XYZ"},
		{"a bare www host", "compare at www.example.com/deal?utm_source=x"},
		{"a link whose path is ten digits", "tracking https://ship.example.com/track/9876543210"},
		{"a short number", "delivery was 4 days late"},
		{"a long order number", "order 987654321012 never arrived"},
		{"a price", "paid 1299 for this"},
		{"no contact details at all", "the pump broke on day two and support has not replied"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PII(tc.in); got != tc.in {
				t.Errorf("PII changed text it should not have\n got %q\nwant %q", got, tc.in)
			}
		})
	}
}

// TestPIIRedactsInsideNoise covers what mention text actually looks like: a
// handle, a link and a phone number in one post. The link and the handle
// survive, the number does not.
func TestPIIRedactsInsideNoise(t *testing.T) {
	const in = "@mamaearth DM me on 9876543210, proof here https://t.co/abc123"
	const want = "@mamaearth DM me on [phone], proof here https://t.co/abc123"

	if got := PII(in); got != want {
		t.Errorf("PII(%q)\n got %q\nwant %q", in, got, want)
	}
}

// TestPIIDocumentsWhatItMisses pins the gap the package doc admits to, so it
// fails loudly if someone later "fixes" it without updating the doc.
func TestPIIDocumentsWhatItMisses(t *testing.T) {
	const in = "919876543210"

	if got := PII(in); got != in {
		t.Errorf("PII now redacts a bare country-code number (%q -> %q). "+
			"That is an improvement, but the package doc says it does not: update the doc.", in, got)
	}
}

func TestPIIIsIdempotent(t *testing.T) {
	const in = "mail priya@example.com or call +91 98765 43210"

	once := PII(in)
	if twice := PII(once); twice != once {
		t.Errorf("a second pass changed the text\n got %q\nwant %q", twice, once)
	}
	if strings.Contains(once, "@example.com") {
		t.Errorf("the email survived: %q", once)
	}
}
