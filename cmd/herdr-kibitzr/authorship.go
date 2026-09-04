package main

// Reports the text one agent has written, so a comment is traced to the agent
// that typed it rather than to whoever happens to be idle in the repository.
//
// The cursor is opaque: each adapter stores whatever it needs to resume, and
// the caller only persists the string it gets back. An empty one must answer
// with no additions and a cursor at the present, so an agent is never blamed
// for what it found on arrival.
type authorship interface {
	additions(cursor string) (added []addition, next string, err error)
}

// For an edit the replacement text, for a new file the whole content, so every
// line of it is a line the agent wrote.
type addition struct {
	path string
	text string
}

// Claude records the literal text of every edit it makes. Nothing else does.
func authorshipFor(finished turn, repo string) authorship {
	if finished.agent == "claude" {
		return claudeLog{sessionID: finished.session}
	}
	return recentlyChanged{repo: repo}
}
