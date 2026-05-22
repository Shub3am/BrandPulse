// prompts_test.go guards the parser, not the prose.
//
// Guardrails is parsed rather than duplicated as a Go literal, which buys one
// source of truth and costs a parser that can go wrong in two silent ways:
// dropping the tail of a wrapped bullet, and swallowing the prose that follows
// a list. Both produce a wrong guardrail list and neither produces an error, so
// bp-responder would just quietly promise a refund. The wrapped-bullet case is
// pinned in TestParseBullets rather than against guardrails.md, because no
// phrase in that file is long enough to wrap and inventing one to test the
// parser would be testing the fixture.
package prompts

import (
	"slices"
	"strings"
	"testing"
)

func TestGuardrailsAreTheThirtyInTheFile(t *testing.T) {
	// The count is asserted so that adding a guardrail to the markdown without
	// this test noticing is impossible: the failure names the new total and
	// the fix is one line.
	if len(Guardrails) != 30 {
		t.Fatalf("Guardrails has %d entries, want 30:\n%s", len(Guardrails), strings.Join(Guardrails, "\n"))
	}

	// One phrase per group in the file, so a whole section going missing fails
	// here rather than in a draft nobody reread.
	mustContain := []string{"refund everyone", "this is our fault", "clinically proven", "our employee", "by tomorrow", "we will sue"}
	for _, phrase := range mustContain {
		if !slices.Contains(Guardrails, phrase) {
			t.Errorf("no guardrail is %q:\n%s", phrase, strings.Join(Guardrails, "\n"))
		}
	}
}

func TestEveryGuardrailIsALowercasePhrase(t *testing.T) {
	// bp-responder lowercases the draft and does a substring match, so an
	// upper-case character in the file is a guardrail that can never fire.
	for _, guardrail := range Guardrails {
		if guardrail != strings.ToLower(guardrail) {
			t.Errorf("guardrail %q is not lower case, so it can never match a draft", guardrail)
		}
	}
}

func TestProseAfterAListIsNotAGuardrail(t *testing.T) {
	// guardrails.md ends with two paragraphs of explanation. A parser that
	// kept appending after the list would glue them to the last bullet.
	for _, guardrail := range Guardrails {
		if strings.Contains(guardrail, "recompile") {
			t.Errorf("a guardrail swallowed the prose after the list: %q", guardrail)
		}
	}
}

func TestParseBullets(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want []string
	}{
		{"no bullets at all", "# Title\n\nJust prose.\n", nil},
		{"two flat bullets", "- one\n- two\n", []string{"one", "two"}},
		{
			name: "a bullet wrapped across two lines",
			md:   "- the first half\n  and the second\n",
			want: []string{"the first half and the second"},
		},
		{
			name: "prose after a blank line ends the list",
			md:   "- only bullet\n\nNot a bullet.\n",
			want: []string{"only bullet"},
		},
		{
			name: "two lists separated by prose",
			md:   "- first list\n\nSome prose.\n\n- second list\n",
			want: []string{"first list", "second list"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseBullets(c.md)
			if len(got) != len(c.want) {
				t.Fatalf("parseBullets = %q, want %q", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("bullet %d = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestLoadReturnsTheFile(t *testing.T) {
	if !strings.Contains(Load("guardrails"), "# Global guardrails") {
		t.Error("Load(\"guardrails\") did not return guardrails.md")
	}
}

func TestLoadPanicsOnAPromptThatIsNotCompiledIn(t *testing.T) {
	// The signature has no error on purpose: the files are embedded, so a
	// missing one is a build mistake every run would hit, not a condition a
	// caller could recover from. This test pins that choice.
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("Load returned normally for a prompt that does not exist")
		}
		if message, _ := recovered.(string); !strings.Contains(message, "bp_nonexistent") {
			t.Errorf("the panic is %v, and it must name the prompt that is missing", recovered)
		}
	}()

	Load("bp_nonexistent")
}
