package main

import "testing"

func TestDecideNudgesOnTheFirstAddedComment(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1"}, "sha1", 1, 1)

	if !nudge {
		t.Error("did not nudge for one added comment")
	}
	if next.NudgedAtCount != 1 {
		t.Errorf("mark %d, want 1", next.NudgedAtCount)
	}
}

func TestDecideStaysQuietWhenTheAgentKeptSomeComments(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 2, 2)

	if nudge {
		t.Error("nudged again after a cleanup left 2 of 5 comments")
	}
	if next.NudgedAtCount != 5 {
		t.Errorf("mark %d, want the high-water mark 5 kept", next.NudgedAtCount)
	}
}

func TestDecideStaysQuietWhenTheAgentIgnoredTheNudge(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 5, 5)

	if nudge {
		t.Error("nudged again for the same 5 comments")
	}
	if next.NudgedAtCount != 5 {
		t.Errorf("mark %d, want 5", next.NudgedAtCount)
	}
}

func TestDecideNudgesAgainWhenNewCommentsArriveAfterACleanup(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 6, 6)

	if !nudge {
		t.Error("did not nudge when the count rose above the mark")
	}
	if next.NudgedAtCount != 6 {
		t.Errorf("mark %d, want 6", next.NudgedAtCount)
	}
}

// A mark counted from the old baseline cannot be compared against the new one.
// Storing it unconverted would swallow every later comment until the count
// climbed past it again.
func TestDecideRebasesTheMarkWhenANudgedTurnCommitted(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1"}, "sha2", 9, 3)

	if !nudge {
		t.Error("did not nudge for 9 added comments")
	}
	if next.Baseline != "sha2" {
		t.Errorf("baseline %q, want %q", next.Baseline, "sha2")
	}
	if next.NudgedAtCount != 3 {
		t.Errorf("mark %d, want the 3 still measurable from sha2", next.NudgedAtCount)
	}
}

func TestDecideRebasesTheMarkWhenAQuietTurnCommitted(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha2", 5, 2)

	if nudge {
		t.Error("nudged for comments already covered by an earlier nudge")
	}
	if next.Baseline != "sha2" {
		t.Errorf("baseline %q, want %q", next.Baseline, "sha2")
	}
	if next.NudgedAtCount != 2 {
		t.Errorf("mark %d, want the 2 still measurable from sha2", next.NudgedAtCount)
	}
}

func TestDecideFirstRunOnlyRecordsTheBaseline(t *testing.T) {
	nudge, next := decide(state{}, "sha1", 7, 7)

	if nudge {
		t.Error("nudged on the first turn end seen for a project")
	}
	if next.Baseline != "sha1" {
		t.Errorf("baseline %q, want %q", next.Baseline, "sha1")
	}
	if next.NudgedAtCount != 0 {
		t.Errorf("mark %d, want 0", next.NudgedAtCount)
	}
}
