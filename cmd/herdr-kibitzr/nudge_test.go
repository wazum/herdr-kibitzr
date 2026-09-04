package main

import (
	"strings"
	"testing"
)

func TestNudgeTextListsTheFilesInAStableOrder(t *testing.T) {
	comments := map[string][]string{
		"src/Repository.php": {"// three"},
		"src/Order.php":      {"// one", "// two"},
		"src/Money.php":      {"// four"},
		"main.go":            {"// five"},
		"internal/db.go":     {"// six"},
	}

	got := nudgeText("Review your comments.", comments, "")

	want := "Review your comments.\n\n" +
		"internal/db.go\nmain.go\nsrc/Money.php\nsrc/Order.php\nsrc/Repository.php"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// An agent that commits before its turn ends is nudged afterwards, so the fix
// would sit uncommitted on top of a commit that still carries the slop.
func TestNudgeTextAsksForAnAmendWhenTheWorkIsAlreadyCommitted(t *testing.T) {
	got := nudgeText("Review your comments.",
		map[string][]string{"src/Order.php": {"// one"}}, "915da5eb")

	if !strings.Contains(got, "915da5eb") {
		t.Errorf("the commit is not named:\n%s", got)
	}
	if !strings.Contains(got, "amend") {
		t.Errorf("no amend asked for:\n%s", got)
	}
}

func TestNudgeTextSaysNothingAboutCommitsWhenThereIsNone(t *testing.T) {
	got := nudgeText("Review your comments.",
		map[string][]string{"a.go": {"// one"}}, "")

	if strings.Contains(got, "amend") {
		t.Errorf("asked for an amend with nothing committed:\n%s", got)
	}
}

func TestNudgeTextKeepsThePromptWhenNoFileIsNamed(t *testing.T) {
	got := nudgeText("Review your comments.", nil, "")

	if got != "Review your comments." {
		t.Errorf("got %q, want the prompt unchanged", got)
	}
}
