package main

import "encoding/json"

type turn struct {
	paneID  string
	cwd     string
	agent   string
	session string
}

// What one `herdr pane get` answers that the event does not: whether somebody
// is looking at the pane, and which agent session is running in it.
type paneFacts struct {
	focused bool
	session string
}

func readPane(paneJSON string) paneFacts {
	var response struct {
		Result struct {
			Pane struct {
				Focused bool `json:"focused"`
				Session struct {
					Value string `json:"value"`
				} `json:"agent_session"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(paneJSON), &response); err != nil {
		return paneFacts{}
	}
	return paneFacts{
		focused: response.Result.Pane.Focused,
		session: response.Result.Pane.Session.Value,
	}
}

// Herdr builds the context from the pane an action was invoked in, so this is
// the label the badge composes from.
func focusedAgent(contextJSON string) string {
	var context struct {
		FocusedPaneAgent string `json:"focused_pane_agent"`
	}
	if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
		return ""
	}
	return context.FocusedPaneAgent
}

func turnEnd(eventJSON, contextJSON string) (turn, bool) {
	var event struct {
		Data struct {
			PaneID      string `json:"pane_id"`
			AgentStatus string `json:"agent_status"`
			Agent       string `json:"agent"`
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

	return turn{
		paneID: event.Data.PaneID,
		cwd:    context.FocusedPaneCwd,
		agent:  event.Data.Agent,
	}, true
}
