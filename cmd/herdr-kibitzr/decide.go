package main

type state struct {
	Baseline      string `json:"baseline"`
	NudgedAtCount int    `json:"nudged_at_count"`
}

// sinceBaseline is what the agent has added since the recorded baseline, and is
// the figure the mark is compared against. sinceHead is the same measurement
// taken from the current commit, and only differs when the agent committed
// during this turn.
func decide(prev state, head string, sinceBaseline, sinceHead int) (bool, state) {
	if prev.Baseline == "" {
		return false, state{Baseline: head}
	}
	nudge := sinceBaseline > prev.NudgedAtCount
	if nudge && prev.Baseline == head {
		return true, state{Baseline: head, NudgedAtCount: sinceBaseline}
	}
	return nudge, prev.advance(head, sinceHead)
}

// The mark and the baseline have to name the same revision. Moving the baseline
// therefore restates the mark in terms of the new one: everything still visible
// from head has either just been nudged for or was covered by an earlier nudge.
func (prev state) advance(head string, sinceHead int) state {
	if prev.Baseline != head {
		return state{Baseline: head, NudgedAtCount: sinceHead}
	}
	return state{Baseline: head, NudgedAtCount: prev.NudgedAtCount}
}
