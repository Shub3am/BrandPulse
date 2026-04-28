// Package prompts holds every LLM prompt in the repo as an embedded markdown
// file, loaded by name so no prompt is ever a string literal in an agent.
//
// Adding a prompt is a recompile: the loader reads the embedded filesystem, not
// the disk, so a new .md file that has not been built is not loadable.
//
// This package belongs to B1 (their Task 9). It is implemented here so track
// B4's responder and briefer can load their prompts and read the guardrail
// list. guardrails.md is B1's; responder.md and briefer.md are B4's, per
// CONTRACTS §4.
package prompts

import (
	"embed"
	"strings"
)

//go:embed *.md
var files embed.FS

// Guardrails is the global list of things no drafted reply may say, parsed
// from guardrails.md's bullet lines at init.
//
// It is parsed rather than duplicated as a Go literal so there is one source of
// truth and the text stays editable by whoever tunes the responder.
var Guardrails = parseGuardrails(Load("guardrails"))

// Load returns the contents of <name>.md.
//
// It returns no error and panics on an unknown name on purpose: a missing
// prompt is a build mistake, not a runtime condition, and a prompt that
// silently resolves to "" is an LLM call with no instructions.
func Load(name string) string {
	b, err := files.ReadFile(name + ".md")
	if err != nil {
		panic("prompts: no embedded prompt named " + name + ".md")
	}
	return string(b)
}

// parseGuardrails takes the text after "- " on every bullet line, ignoring
// headings and prose, so the file reads as documentation and parses as data.
func parseGuardrails(doc string) []string {
	var out []string
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		if phrase := strings.TrimSpace(trimmed[2:]); phrase != "" {
			out = append(out, phrase)
		}
	}
	return out
}
