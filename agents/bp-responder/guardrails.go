// The guardrail check, as two pure functions over strings.
//
// It lives apart from the handler so it can be tested without an LLM stub: the
// question "does this text contain a banned phrase" has nothing to do with
// where the text came from. The regenerate-once loop in main.go is the only
// part that needs a model.
//
// This file must not grow a way to send text anywhere. Nothing in this repo
// posts to any platform.

package main

import (
	"regexp"
	"strings"
)

// violationsIn returns the banned phrases present in text, in the order the
// banned list gives them.
//
// Matching is case-insensitive and on substrings, not words: a model that
// writes "We Will Refund Everyone." has made exactly the commitment the phrase
// forbids, and punctuation does not change that.
func violationsIn(text string, banned []string) []string {
	haystack := strings.ToLower(text)
	seen := map[string]bool{}
	var found []string

	for _, phrase := range banned {
		needle := strings.ToLower(strings.TrimSpace(phrase))
		if needle == "" || seen[needle] {
			continue
		}
		if strings.Contains(haystack, needle) {
			seen[needle] = true
			found = append(found, phrase)
		}
	}
	return found
}

// strip removes every phrase from text and tidies up what that leaves behind.
//
// It is the last resort, used only when a regenerated draft still contains a
// banned phrase. The result is deliberately a bit mangled: a human has to read
// and edit this draft before it goes anywhere, and a sentence that reads oddly
// is a better outcome than one that fluently promises a refund.
func strip(text string, phrases []string) string {
	for _, phrase := range phrases {
		text = removeFold(text, strings.TrimSpace(phrase))
	}
	return tidy(text)
}

// removeFold deletes every case-insensitive occurrence of phrase from text
// while preserving the casing of everything around it, which strings.Replacer
// cannot do.
func removeFold(text, phrase string) string {
	if phrase == "" {
		return text
	}
	for {
		at := strings.Index(strings.ToLower(text), strings.ToLower(phrase))
		if at < 0 {
			return text
		}
		text = text[:at] + text[at+len(phrase):]
	}
}

var (
	collapseSpaces  = regexp.MustCompile(`[ \t]{2,}`)
	spaceBeforePunc = regexp.MustCompile(`\s+([.,!?;:])`)
	repeatedPunc    = regexp.MustCompile(`([.,!?;:])\s*([.,!?;:])+`)
	emptySentence   = regexp.MustCompile(`(?m)^\s*[.,!?;:]\s*`)
)

// tidy cleans the double spaces and stranded punctuation a removal leaves.
func tidy(text string) string {
	text = collapseSpaces.ReplaceAllString(text, " ")
	text = spaceBeforePunc.ReplaceAllString(text, "$1")
	text = repeatedPunc.ReplaceAllString(text, "$1")
	text = emptySentence.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}
