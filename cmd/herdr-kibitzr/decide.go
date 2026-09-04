package main

// How far kibitzr has read this agent's writes, whether the agent still owes a
// turn to act on the last nudge, and the status it was last seen in.
type state struct {
	Cursor          string `json:"cursor"`
	AwaitingCleanup bool   `json:"awaiting_cleanup"`
	LastStatus      string `json:"last_status"`
}

// Herdr sends the same event when a pane's title or token counts change, with
// the status unchanged. Only a change of status ends a turn, and acting on the
// rest is what dropped a prompt into a composer somebody was typing in.
func settled(prev state, status string) bool {
	return prev.LastStatus != status
}

func record(next state, status string) state {
	next.LastStatus = status
	return next
}

// count is what this agent wrote since the cursor, so every comment in it is
// one nobody has been asked about yet.
func decide(prev state, cursor string, count int) (bool, state) {
	// Acting on a nudge means rewriting comments, and a rewrite is text the
	// agent just wrote, so counting this turn would nudge about the cleanup.
	// Only a turn that wrote something can be that reply. A turn that wrote
	// nothing keeps the pass, because herdr also reports a turn end when a
	// pane's title or tokens change.
	if prev.AwaitingCleanup {
		return false, state{Cursor: cursor, AwaitingCleanup: count == 0}
	}
	if count > 0 {
		return true, state{Cursor: cursor, AwaitingCleanup: true}
	}
	return false, state{Cursor: cursor}
}
