// Package redact strips personal contact details out of mention text before it
// reaches an LLM prompt.
//
// Public handles and URLs are deliberately left alone: they are the data, and a
// dashboard that cannot link back to the post it is quoting is useless.
//
// This package belongs to B1 (their Task 3). It is implemented here so track
// B4's responder can satisfy the "redact before the prompt" rule before B1
// lands. Replace it with B1's version when that arrives.
package redact

import "regexp"

// Compiled once at package level: this runs on every mention text in every run.
var (
	emailPattern = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)

	// Indian mobile numbers, with or without the +91 country code and with
	// space or hyphen grouping: "+91 98765 43210", "+91-98765-43210",
	// "9876543210". The leading boundary stops it eating the tail of a longer
	// digit run such as an order id.
	phonePattern = regexp.MustCompile(`(?:\+91[\s-]?)?\b[6-9]\d{4}[\s-]?\d{5}\b`)
)

// PII replaces email addresses with [email] and phone numbers with [phone].
//
// Emails go first: an address contains an @ and would otherwise be mistaken for
// a handle by anything reading the output.
func PII(text string) string {
	text = emailPattern.ReplaceAllString(text, "[email]")
	return phonePattern.ReplaceAllString(text, "[phone]")
}
