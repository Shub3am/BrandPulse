// prompts_test.go guards the parser, not the prose.
//
// Guardrails is parsed rather than duplicated as a Go literal, which buys one
// source of truth and costs a parser that can go wrong in two silent ways:
// dropping the tail of a wrapped bullet, and swallowing the prose that follows
// a list. Both produce a shorter guardrail list and neither produces an error,
// so bp-responder would just quietly promise a refund.
package prompts

import (
	"strings"
	"testing"
)

func TestGuardrailsAreTheSixInTheFile(t *testing.T) {
	// The count is asserted so that adding a guardrail to the markdown without
	// this test noticing is impossible: the failure names the new total and
	// the fix is one line.
	if len(Guardrails) != 6 {
		t.Fatalf("Guardrails has %d entries, want 6:\n%s", len(Guardrails), strings.Join(Guardrails, "\n"))
	}

	mustMention := []string{"refund", "fault", "medical", "employee", "deadline", "negligent"}
	for _, subject := range mustMention {
		found := false
		for _, guardrail := range Guardrails {
			if strings.Contains(guardrail, subject) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no guardrail mentions %q:\n%s", subject, strings.Join(Guardrails, "\n"))
		}
	}
}

func TestAWrappedBulletKeepsItsTail(t *testing.T) {
	// The legal-characterisation bullet wraps in guardrails.md. If the
	// continuation line were dropped, the guardrail would read "Never
	// characterise anything in legal terms, such as saying the brand was" and
	// still look plausible in a diff.
	var legal string
	for _, guardrail := range Guardrails {
		if strings.Contains(guardrail, "legal terms") {
			legal = guardrail
		}
	}
	if legal == "" {
		t.Fatal("the legal-characterisation guardrail is missing entirely")
	}
	if !strings.HasSuffix(legal, "negligent.") {
		t.Errorf("the wrapped bullet is %q, which lost its continuation line", legal)
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
	if !strings.Contains(Load("guardrails"), "# Response guardrails") {
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
