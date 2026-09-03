package main

import "testing"

func TestDecideNudgesOnTheFirstAddedComment(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1"}, "sha1", 1)

	if !nudge {
		t.Error("did not nudge for one added comment")
	}
	if next.NudgedAtCount != 1 {
		t.Errorf("mark %d, want 1", next.NudgedAtCount)
	}
}

func TestDecideStaysQuietWhenTheAgentKeptSomeComments(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 2)

	if nudge {
		t.Error("nudged again after a cleanup left 2 of 5 comments")
	}
	if next.NudgedAtCount != 5 {
		t.Errorf("mark %d, want the high-water mark 5 kept", next.NudgedAtCount)
	}
}

func TestDecideStaysQuietWhenTheAgentIgnoredTheNudge(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 5)

	if nudge {
		t.Error("nudged again for the same 5 comments")
	}
	if next.NudgedAtCount != 5 {
		t.Errorf("mark %d, want 5", next.NudgedAtCount)
	}
}

func TestDecideNudgesAgainWhenNewCommentsArriveAfterACleanup(t *testing.T) {
	nudge, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha1", 6)

	if !nudge {
		t.Error("did not nudge when the count rose above the mark")
	}
	if next.NudgedAtCount != 6 {
		t.Errorf("mark %d, want 6", next.NudgedAtCount)
	}
}

func TestDecideResetsTheMarkWhenTheAgentCommitted(t *testing.T) {
	// The count came from the old baseline, so it says nothing about what the
	// new one already contains.
	_, next := decide(state{Baseline: "sha1", NudgedAtCount: 5}, "sha2", 5)

	if next.Baseline != "sha2" {
		t.Errorf("baseline %q, want %q", next.Baseline, "sha2")
	}
	if next.NudgedAtCount != 0 {
		t.Errorf("mark %d, want 0 against the new baseline", next.NudgedAtCount)
	}
}

func TestDecideFirstRunOnlyRecordsTheBaseline(t *testing.T) {
	nudge, next := decide(state{}, "sha1", 7)

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
