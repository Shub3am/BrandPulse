// What the corpus is made of is tested in demo/crisis, beside it. What is
// tested here is this command: that its dry run is safe and that its real path
// refuses to pretend without a database.

package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"brandpulse/demo/crisis"
	"brandpulse/internal/models"
)

// CI runs this path. It must not need DATABASE_URL, a container or a network.
func TestDryRunWritesTheShapeAndTouchesNothing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	if err := run(context.Background(), "brd_demo", true, false, &out); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	printed := out.String()
	for _, want := range []string{
		"dry run",
		"nothing written",
		"brd_demo",
		fmt.Sprintf("%d synthetic mentions", crisis.Count),
		// Only that the line is there; the count is the corpus's claim and
		// demo/crisis asserts it.
		"followers:",
		string(models.SourceX),
		string(models.SourceReddit),
		string(models.SourceInstagram),
		crisis.SyntheticKey,
	} {
		if !strings.Contains(printed, want) {
			t.Errorf("dry run output is missing %q:\n%s", want, printed)
		}
	}
}

// -dry-run wins over -cleanup. A dry run that deletes rows is the worst
// possible reading of the flag, and the flag is typed by hand on stage.
func TestDryRunWinsOverCleanup(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	if err := run(context.Background(), "brd_demo", true, true, &out); err != nil {
		t.Fatalf("dry run with cleanup: %v", err)
	}
	if strings.Contains(out.String(), "removed") {
		t.Errorf("a dry run reported a deletion:\n%s", out.String())
	}
}

func TestWithoutDryRunTheCommandNeedsADatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	err := run(context.Background(), "brd_demo", false, false, &out)
	if err == nil {
		t.Fatal("a real injection with no DATABASE_URL reported success")
	}
	if out.Len() != 0 {
		t.Errorf("it printed something before failing:\n%s", out.String())
	}
}
