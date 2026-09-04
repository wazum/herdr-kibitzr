package main

type state struct {
	Cursor          string `json:"cursor"`
	AwaitingCleanup bool   `json:"awaiting_cleanup"`
	LastStatus      string `json:"last_status"`
}

// Herdr sends the same event when a pane's title or tokens change. Acting on
// those is what dropped a prompt into a composer somebody was typing in.
func settled(prev state, status string) bool {
	return prev.LastStatus != status
}

func record(next state, status string) state {
	next.LastStatus = status
	return next
}

// count covers only what this agent wrote since the cursor, so every comment
// in it is one nobody has been asked about yet.
func decide(prev state, cursor string, count int) (bool, state) {
	// A rewritten comment is text the agent just wrote, so counting the turn
	// that acts on a nudge would nudge about the cleanup. Only a turn that
	// wrote something can be that reply.
	if prev.AwaitingCleanup {
		return false, state{Cursor: cursor, AwaitingCleanup: count == 0}
	}
	if count > 0 {
		return true, state{Cursor: cursor, AwaitingCleanup: true}
	}
	return false, state{Cursor: cursor}
}
