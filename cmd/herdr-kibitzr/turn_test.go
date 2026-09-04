package main

import "testing"

const doneEvent = `{"event":"pane_agent_status_changed","data":{"type":"pane_agent_status_changed","pane_id":"w1:p2","workspace_id":"w1","agent_status":"done","agent":"claude"}}`

const paneContext = `{"focused_pane_id":"w1:p2","focused_pane_cwd":"/tmp/project","workspace_id":"w1"}`

func TestReadPaneTakesFocusAndSessionFromOneAnswer(t *testing.T) {
	answer := `{"result":{"pane":{"pane_id":"w1:p2","focused":true,` +
		`"agent_session":{"agent":"claude","kind":"id","value":"abc-123"}}}}`

	got := readPane(answer)

	if !got.focused {
		t.Error("focused was not read")
	}
	if got.session != "abc-123" {
		t.Errorf("session %q, want %q", got.session, "abc-123")
	}
}

func TestReadPaneSurvivesAnAnswerItCannotUse(t *testing.T) {
	for name, answer := range map[string]string{
		"no session": `{"result":{"pane":{"pane_id":"w1:p2","focused":false}}}`,
		"no pane":    `{"result":{}}`,
		"not json":   `nope`,
	} {
		got := readPane(answer)
		if got.focused || got.session != "" {
			t.Errorf("%s: got %+v, want the zero value", name, got)
		}
	}
}

func TestTurnEndAcceptsOnlySettledStatuses(t *testing.T) {
	accepted := map[string]bool{"done": true, "idle": true}

	for _, status := range []string{"done", "idle", "working", "blocked", "unknown", ""} {
		event := `{"data":{"pane_id":"w1:p2","agent_status":"` + status + `"}}`

		if _, ok := turnEnd(event, paneContext); ok != accepted[status] {
			t.Errorf("status %q accepted=%v, want %v", status, ok, accepted[status])
		}
	}
}

func TestTurnEndRejectsAContextForAnotherPane(t *testing.T) {
	otherPane := `{"focused_pane_id":"w1:p9","focused_pane_cwd":"/elsewhere"}`

	if _, ok := turnEnd(doneEvent, otherPane); ok {
		t.Error("accepted a cwd belonging to a different pane")
	}
}

func TestTurnEndRejectsAPaneWithoutACwd(t *testing.T) {
	noCwd := `{"focused_pane_id":"w1:p2"}`

	if _, ok := turnEnd(doneEvent, noCwd); ok {
		t.Error("accepted a pane with no working directory")
	}
}

func TestTurnEndAcceptsAFinishedAgent(t *testing.T) {
	got, ok := turnEnd(doneEvent, paneContext)

	if !ok {
		t.Fatal("rejected a done event")
	}
	if got.paneID != "w1:p2" {
		t.Errorf("pane %q, want %q", got.paneID, "w1:p2")
	}
	if got.cwd != "/tmp/project" {
		t.Errorf("cwd %q, want %q", got.cwd, "/tmp/project")
	}
	// The kind, not the current label, so two nudges cannot stack two badges.
	if got.agent != "claude" {
		t.Errorf("agent %q, want %q", got.agent, "claude")
	}
}
