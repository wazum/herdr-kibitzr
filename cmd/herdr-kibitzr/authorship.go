package main

// Reports the text one agent has written. This is how kibitzr blames the agent
// that typed a comment and not whoever happens to be idle in the same repo.
//
// Each adapter decides what its own cursor means and keeps whatever it needs to
// carry on from there. The caller only stores the string. An empty cursor must
// return no additions and a cursor at the present, so an agent never answers
// for what it found when it arrived.
type authorship interface {
	additions(cursor string) (added []addition, next string, err error)
}

// For an edit the replacement text, for a new file the whole content.
//
// An edit replaces a region, so text carries the unchanged lines around the
// change as well. replaced holds what was there before, and a line in both is
// a line the agent only carried along.
type addition struct {
	path     string
	text     string
	replaced string
}

// Claude records the literal text of every edit it makes. Nothing else does.
func authorshipFor(finished *turn, repo string) authorship {
	if finished.agent == "claude" {
		return claudeLog{sessionID: finished.session}
	}
	return recentlyChanged{repo: repo}
}
