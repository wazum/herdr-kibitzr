package main

import "testing"

// Herdr fires the same event when a pane's title or token counts change, with
// the status unchanged. Those are not turn ends, and acting on them is what
// dropped a prompt into a composer somebody was typing in.
func TestSettledOnlyOnAChangeOfStatus(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		last   string
		now    string
		settle bool
	}{
		{"finished working", "working", "done", true},
		{"finished and then seen", "done", "idle", true},
		{"first status ever seen", "", "idle", true},
		{"title changed while done", "done", "done", false},
		{"tokens changed while idle", "idle", "idle", false},
	} {
		if got := settled(state{LastStatus: testCase.last}, testCase.now); got != testCase.settle {
			t.Errorf("%s: %q to %q gave %v, want %v",
				testCase.name, testCase.last, testCase.now, got, testCase.settle)
		}
	}
}

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

// Herdr reports a turn end for a presentation change too, so an event can
// arrive between the nudge and the agent's reply. Letting that spend the pass
// leaves the real cleanup to be nudged about.
func TestDecideKeepsTheCleanupPassForATurnWithNoWrites(t *testing.T) {
	_, next := decide(state{Cursor: "20", AwaitingCleanup: true}, "30", 0)

	if !next.AwaitingCleanup {
		t.Error("a turn that wrote nothing spent the cleanup pass")
	}
	if next.Cursor != "30" {
		t.Errorf("cursor %q, want the turn marked read anyway", next.Cursor)
	}
}

func TestDecideNudgesAgainAfterTheCleanupTurnIsSpent(t *testing.T) {
	_, spent := decide(state{Cursor: "20", AwaitingCleanup: true}, "30", 3)

	nudge, _ := decide(spent, "40", 2)

	if !nudge {
		t.Error("did not nudge for comments written after the cleanup")
	}
}
