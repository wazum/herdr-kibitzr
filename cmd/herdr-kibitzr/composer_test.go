package main

import "testing"

// Captured from a real Claude Code pane. Empty, it shows a dim suggestion; with
// something typed, the text carries no styling at all.
const (
	claudeEmpty = "  -- INSERT --\n❯\u00a0\x1b[0m\x1b[2mgo build ./...\x1b[0m\r\n───────\n"
	claudeTyped = "  -- INSERT --\n❯\u00a0review the uncommitted\r\n───────\n"
)

func TestClaudeComposerTellsATypedLineFromASuggestion(t *testing.T) {
	if typedInto(claudeEmpty) {
		t.Error("a dim suggestion was read as something somebody typed")
	}
	if !typedInto(claudeTyped) {
		t.Error("typed text was read as an empty composer")
	}
}

func TestClaudeComposerReadsTheLastPromptLine(t *testing.T) {
	// A transcript can quote an earlier prompt, so the composer is the last one.
	screen := "❯\u00a0an older prompt from the transcript\r\n" + claudeEmpty

	if typedInto(screen) {
		t.Error("an earlier prompt in the scrollback was read as the composer")
	}
}

func TestClaudeComposerCountsPartlyDimLinesAsTyped(t *testing.T) {
	// Claude shows the rest of a suggestion dim while you type over the front
	// of it, so any undimmed text means somebody is mid-sentence.
	screen := "❯\u00a0go bu\x1b[2mild ./...\x1b[0m\r\n"

	if !typedInto(screen) {
		t.Error("text typed over a suggestion was read as an empty composer")
	}
}

func TestClaudeComposerSaysNothingWhenItCannotFindTheLine(t *testing.T) {
	for name, screen := range map[string]string{
		"no marker at all": "just some output\nand more\n",
		"nothing at all":   "",
		"marker alone":     "❯\u00a0\r\n",
	} {
		if typedInto(screen) {
			t.Errorf("%s: claimed somebody was typing", name)
		}
	}
}
