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
// STUB: signatures only, bodies panic. B1 Task 3 implements this.
package redact

// PII returns text with email addresses replaced by "[email]" and phone
// numbers by "[phone]". Indian formats are in scope: "+91 98765 43210",
// "9876543210" and "+91-98765-43210" all redact.
func PII(text string) string {
	panic("not implemented")
}
