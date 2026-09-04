package main

import (
	"path/filepath"
	"testing"
)

func TestPanesAreNotMutedUntilSomebodySaysSo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "muted.json")

	if muted(path, "w1:p2") {
		t.Error("a pane nobody has touched came back muted")
	}
}

func TestToggleMutesAndUnmutesOnePane(t *testing.T) {
	path := filepath.Join(t.TempDir(), "muted.json")

	now, err := toggleMute(path, "w1:p2")
	if err != nil {
		t.Fatalf("toggleMute: %v", err)
	}
	if !now {
		t.Error("first toggle did not mute")
	}
	if !muted(path, "w1:p2") {
		t.Error("the mute did not survive being written")
	}

	now, err = toggleMute(path, "w1:p2")
	if err != nil {
		t.Fatalf("toggleMute: %v", err)
	}
	if now {
		t.Error("second toggle did not unmute")
	}
	if muted(path, "w1:p2") {
		t.Error("the pane is still muted after being unmuted")
	}
}

func TestMutingOnePaneLeavesTheOthersAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "muted.json")

	if _, err := toggleMute(path, "w1:p2"); err != nil {
		t.Fatal(err)
	}
	if _, err := toggleMute(path, "w9:p7"); err != nil {
		t.Fatal(err)
	}
	if _, err := toggleMute(path, "w1:p2"); err != nil {
		t.Fatal(err)
	}

	if muted(path, "w1:p2") {
		t.Error("w1:p2 should be unmuted again")
	}
	if !muted(path, "w9:p7") {
		t.Error("w9:p7 lost its mute when another pane was toggled")
	}
}
