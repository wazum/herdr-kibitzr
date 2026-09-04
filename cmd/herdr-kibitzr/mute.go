package main

import (
	"path/filepath"
	"slices"
)

func muteFile(stateDir string) string {
	return filepath.Join(stateDir, "muted.json")
}

// Every pane is watched until somebody says otherwise, so a pane missing from
// the muted list gets nudged.
func muted(path, paneID string) bool {
	return listed(path, paneID)
}

func toggleMute(path, paneID string) (bool, error) {
	panes, err := readPanes(path)
	if err != nil {
		return false, err
	}

	if index := slices.Index(panes, paneID); index >= 0 {
		return false, writeJSON(path, slices.Delete(panes, index, index+1))
	}
	return true, writeJSON(path, append(panes, paneID))
}

func listed(path, paneID string) bool {
	panes, err := readPanes(path)
	if err != nil {
		return false
	}
	return slices.Contains(panes, paneID)
}

func readPanes(path string) ([]string, error) {
	var panes []string
	if err := readJSON(path, &panes); err != nil {
		return nil, err
	}
	slices.Sort(panes)
	return panes, nil
}
