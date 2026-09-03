package main

import "testing"

func TestNudgeTextListsTheFilesInAStableOrder(t *testing.T) {
	comments := map[string][]string{
		"src/Repository.php": {"// three"},
		"src/Order.php":      {"// one", "// two"},
		"src/Money.php":      {"// four"},
		"main.go":            {"// five"},
		"internal/db.go":     {"// six"},
	}

	got := nudgeText("Review your comments.", comments)

	want := "Review your comments.\n\n" +
		"internal/db.go\nmain.go\nsrc/Money.php\nsrc/Order.php\nsrc/Repository.php"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestNudgeTextKeepsThePromptWhenNoFileIsNamed(t *testing.T) {
	got := nudgeText("Review your comments.", nil)

	if got != "Review your comments." {
		t.Errorf("got %q, want the prompt unchanged", got)
	}
}
