package main

import "encoding/json"

type turn struct {
	paneID string
	cwd    string
}

func turnEnd(eventJSON, contextJSON string) (turn, bool) {
	var event struct {
		Data struct {
			PaneID      string `json:"pane_id"`
			AgentStatus string `json:"agent_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return turn{}, false
	}

	if event.Data.AgentStatus != "done" && event.Data.AgentStatus != "idle" {
		return turn{}, false
	}

	var context struct {
		FocusedPaneID  string `json:"focused_pane_id"`
		FocusedPaneCwd string `json:"focused_pane_cwd"`
	}
	if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
		return turn{}, false
	}

	// Reading one pane's repository and prompting another would nudge an agent
	// about someone else's work.
	if context.FocusedPaneID != event.Data.PaneID {
		return turn{}, false
	}
	if event.Data.PaneID == "" || context.FocusedPaneCwd == "" {
		return turn{}, false
	}

	return turn{paneID: event.Data.PaneID, cwd: context.FocusedPaneCwd}, true
}
