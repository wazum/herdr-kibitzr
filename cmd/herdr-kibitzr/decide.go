package main

type state struct {
	Baseline      string `json:"baseline"`
	NudgedAtCount int    `json:"nudged_at_count"`
}

func decide(prev state, head string, count int) (bool, state) {
	if prev.Baseline == "" {
		return false, state{Baseline: head}
	}
	if count > prev.NudgedAtCount {
		return true, state{Baseline: head, NudgedAtCount: count}
	}
	return false, prev.advance(head)
}

// A count measured from the old baseline says nothing about what a new one
// already contains, so a moved baseline drops the mark.
func (prev state) advance(head string) state {
	if prev.Baseline != head {
		return state{Baseline: head}
	}
	return state{Baseline: head, NudgedAtCount: prev.NudgedAtCount}
}
