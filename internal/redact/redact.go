// Package redact strips personal contact details out of mention text before it
// reaches an LLM prompt.
//
// It removes emails and phone numbers. It deliberately leaves public handles
// and URLs alone: those are the data, they are already public, and the
// responder needs them to write a reply that makes sense.
//
// This is not the model's job and it is not optional. Every agent that builds
// a prompt from Mention.Text runs it first.
//
// # What it does not catch
//
// A bare "919876543210", country code run together with the number and no
// plus, is left alone. Twelve digits with no separator and no plus is equally
// likely to be an order number, and this function has no context to tell them
// apart. Write "+91" and it redacts.
package redact

import (
	"regexp"
	"strings"
)

// The three things worth finding in a mention. Compiled once at package level:
// this runs on every mention text in every run.
const (
	// urlSource is checked first so a link is returned untouched even when it
	// carries something that looks like a phone number in its path.
	urlSource = `https?://\S+|www\.\S+`

	emailSource = `[\w.%+\-]+@[\w.\-]+\.[a-zA-Z]{2,}`

	// phoneSource is two cases. With an explicit +91 the plus supplies the
	// left-hand boundary, so the ten digits may run together. Without it, the
	// number needs a word boundary at both ends, which is what stops a
	// fourteen-digit order number from losing ten of its digits.
	phoneSource = `\+91[-.\s]?[6-9]\d{4}[-.\s]?\d{5}\b|\b[6-9]\d{4}[-.\s]?\d{5}\b`
)

var (
	urlPattern   = regexp.MustCompile(urlSource)
	emailPattern = regexp.MustCompile(emailSource)

	// piiPattern is one pass over the text. Go's regexp prefers the leftmost
	// match and, among those, the earliest alternative, so a URL wins over the
	// phone number inside it.
	piiPattern = regexp.MustCompile(strings.Join([]string{urlSource, emailSource, phoneSource}, "|"))
)

// PII returns text with email addresses replaced by "[email]" and phone
// numbers by "[phone]". Indian formats are in scope: "+91 98765 43210",
// "9876543210" and "+91-98765-43210" all redact.
func PII(text string) string {
	return piiPattern.ReplaceAllStringFunc(text, func(match string) string {
		switch {
		case urlPattern.MatchString(match):
			return match
		case emailPattern.MatchString(match):
			return "[email]"
		default:
			return "[phone]"
		}
	})
}
