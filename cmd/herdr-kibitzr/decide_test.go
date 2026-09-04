package main

import "testing"

func TestDecideNudgesOnTheFirstAddedComment(t *testing.T) {
	nudge, next := decide(state{Cursor: "10"}, "20", 1)

	if !nudge {
		t.Error("did not nudge for one added comment")
	}
	if next.Cursor != "20" {
		t.Errorf("cursor %q, want %q", next.Cursor, "20")
	}
	if !next.AwaitingCleanup {
		t.Error("a nudge has to grant the next turn its cleanup pass")
	}
}

func TestDecideStaysQuietWhenTheAgentWroteNoComments(t *testing.T) {
	nudge, next := decide(state{Cursor: "10"}, "20", 0)

	if nudge {
		t.Error("nudged about nothing")
	}
	if next.Cursor != "20" {
		t.Errorf("cursor %q, want the turn to be marked read", next.Cursor)
	}
	if next.AwaitingCleanup {
		t.Error("a quiet turn owes the agent nothing")
	}
}

// The loop that mattered in the field: the reply to a nudge is itself written
// text, so judging that turn by its count nudges about the cleanup.
func TestDecideGivesTheAgentOneUncontestedCleanupTurn(t *testing.T) {
	nudge, next := decide(state{Cursor: "20", AwaitingCleanup: true}, "30", 3)

	if nudge {
		t.Error("nudged about the comments the previous nudge produced")
	}
	if next.AwaitingCleanup {
		t.Error("the cleanup pass was not spent")
	}
	if next.Cursor != "30" {
		t.Errorf("cursor %q, want the cleanup turn marked read", next.Cursor)
	}
}

func TestDecideNudgesAgainAfterTheCleanupTurnIsSpent(t *testing.T) {
	_, spent := decide(state{Cursor: "20", AwaitingCleanup: true}, "30", 3)

	nudge, _ := decide(spent, "40", 2)

	if !nudge {
		t.Error("did not nudge for comments written after the cleanup")
	}
}
