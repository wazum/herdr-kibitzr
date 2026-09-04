package main

// How far an agent's writes have been read, and whether it is owed a turn to
// act on the last nudge.
type state struct {
	Cursor          string `json:"cursor"`
	AwaitingCleanup bool   `json:"awaiting_cleanup"`
}

// count is what this agent wrote since the cursor, so every comment in it is
// one nobody has been asked about yet.
func decide(prev state, cursor string, count int) (bool, state) {
	// Acting on a nudge means rewriting comments, and a rewrite is text the
	// agent just wrote. Counting this turn would nudge about the cleanup.
	if prev.AwaitingCleanup {
		return false, state{Cursor: cursor}
	}
	if count > 0 {
		return true, state{Cursor: cursor, AwaitingCleanup: true}
	}
	return false, state{Cursor: cursor}
}
