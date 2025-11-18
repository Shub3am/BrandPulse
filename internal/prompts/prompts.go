// Package prompts holds every LLM prompt in the repository as an embedded
// markdown file, loaded by name.
//
// A prompt is never inlined in a main.go. Keeping them here means the person
// tuning the responder edits one .md file instead of hunting a string literal
// across nine agent directories, and it means a prompt change shows up in a
// diff as prose.
//
// Each agent's prompt file is owned by that agent's track, not by B1. Adding
// one is a recompile: the loader reads the embedded copy, not the file on
// disk, so editing a .md without rebuilding changes nothing.
//
// guardrails.md is B1's and it is also what keeps this package compiling: a
// //go:embed *.md pattern that matches nothing is a compile error, and B1 owns
// no agent prompt.
package prompts

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.md
var files embed.FS

// Guardrails are the phrases no generated reply may contain, parsed from the
// bullet lines of guardrails.md at init.
//
// It is parsed rather than duplicated as a Go literal so there is one source
// of truth and the text stays editable by whoever tunes the responder.
var Guardrails = parseBullets(Load("guardrails"))

// Load returns the contents of <name>.md.
//
// It returns no error and panics on an unknown name, which is correct: the
// files are compiled in, so a missing one is a build mistake that every run
// would hit, not a runtime condition a caller could handle.
func Load(name string) string {
	b, err := files.ReadFile(name + ".md")
	if err != nil {
		panic(fmt.Sprintf("prompts: no embedded prompt %q: %v", name, err))
	}
	return string(b)
}

// parseBullets returns the text of every "- " bullet in md, joined across the
// continuation lines that a wrapped bullet spans.
func parseBullets(md string) []string {
	var bullets []string
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- "):
			bullets = append(bullets, strings.TrimSpace(trimmed[2:]))
		case trimmed != "" && len(bullets) > 0 && strings.HasPrefix(line, " "):
			// A wrapped bullet: indented, non-blank, and not a new bullet.
			bullets[len(bullets)-1] += " " + trimmed
		default:
			// A blank line ends the list, so prose after it cannot be
			// appended to the last bullet.
			if trimmed == "" && len(bullets) > 0 {
				bullets = append(bullets, "")
			}
		}
	}
	return compact(bullets)
}

// compact drops the empty separators parseBullets used to close a list.
func compact(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
