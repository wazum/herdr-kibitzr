package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "project.json")

	if err := saveState(path, state{Cursor: "42", AwaitingCleanup: true}); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if loaded.Cursor != "42" || !loaded.AwaitingCleanup {
		t.Errorf("got %+v, want cursor 42 and a pending cleanup", loaded)
	}
}

func TestLoadStateTreatsAMissingFileAsAFreshProject(t *testing.T) {
	loaded, err := loadState(filepath.Join(t.TempDir(), "never-written.json"))
	if err != nil {
		t.Fatalf("a project nobody has seen is not an error: %v", err)
	}
	if loaded != (state{}) {
		t.Errorf("got %+v, want the zero state", loaded)
	}
}

// Truncated state read as a fresh project would silently discard the window the
// nudge rule depends on, so it has to be reported rather than assumed.
func TestLoadStateReportsUnreadableState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.json")
	if err := os.WriteFile(path, []byte(`{"cursor":"42","awaiting`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadState(path); err == nil {
		t.Error("truncated state was accepted as a fresh project")
	}
}

func TestSaveStateLeavesNoPartialFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	if err := saveState(path, state{Cursor: "7"}); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "project.json" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("directory holds %v, want only project.json", names)
	}
}
