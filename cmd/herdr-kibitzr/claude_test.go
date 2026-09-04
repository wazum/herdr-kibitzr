package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const claudeEdit = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/src/Order.php","old_string":"private int $total;","new_string":"/** @var int */\nprivate int $total;"}}]}}`

const claudeWrite = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/repo/src/New.php","content":"<?php\n// fresh\nclass New {}\n"}}]}}`

const claudeRead = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/Order.php"}}]}}`

func TestClaudeLogReportsOnlyWhatWasWritten(t *testing.T) {
	log := writeLog(t, claudeEdit, claudeRead, claudeWrite)

	added, next, err := log.additions("0")
	if err != nil {
		t.Fatalf("additions: %v", err)
	}

	want := []addition{
		{
			path:     "/repo/src/Order.php",
			text:     "/** @var int */\nprivate int $total;",
			replaced: "private int $total;",
		},
		{path: "/repo/src/New.php", text: "<?php\n// fresh\nclass New {}\n"},
	}
	assertAdditions(t, added, want)

	if next == "0" {
		t.Error("the cursor did not move past what was read")
	}
}

// Whatever the agent wrote before kibitzr started watching belongs to the past.
func TestClaudeLogStartsAtThePresent(t *testing.T) {
	log := writeLog(t, claudeEdit, claudeWrite)

	added, next, err := log.additions("")
	if err != nil {
		t.Fatalf("additions: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("reported %d additions from before it was watching", len(added))
	}

	size := strconv.Itoa(len(claudeEdit + "\n" + claudeWrite + "\n"))
	if next != size {
		t.Errorf("cursor %q, want the end of the log at %q", next, size)
	}
}

func TestClaudeLogResumesFromTheCursor(t *testing.T) {
	log := writeLog(t, claudeEdit)

	_, next, err := log.additions("")
	if err != nil {
		t.Fatal(err)
	}
	appendToLog(t, log, claudeWrite)

	added, _, err := log.additions(next)
	if err != nil {
		t.Fatalf("additions: %v", err)
	}

	assertAdditions(t, added, []addition{
		{path: "/repo/src/New.php", text: "<?php\n// fresh\nclass New {}\n"},
	})
}

// A session that was resumed elsewhere, or a log rolled over, leaves a cursor
// past the end. Reporting the whole file as new would blame the agent for its
// entire history.
func TestClaudeLogRecoversFromACursorPastTheEnd(t *testing.T) {
	log := writeLog(t, claudeEdit)

	added, next, err := log.additions("999999")
	if err != nil {
		t.Fatalf("additions: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("reported %d additions after losing its place", len(added))
	}
	if next == "999999" {
		t.Error("the cursor was not brought back to the end of the log")
	}
}

// Claude writes the file on its first tool call, so a session that has only
// just started has no log at all. That is normal, not a failure.
func TestClaudeLogTreatsAMissingLogAsNothingWrittenYet(t *testing.T) {
	added, next, err := claudeLog{root: t.TempDir(), sessionID: "not-started"}.additions("7")
	if err != nil {
		t.Fatalf("a session that has not written yet is not an error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("reported %d additions from a log that does not exist", len(added))
	}
	if next != "7" {
		t.Errorf("cursor %q, want the previous %q kept", next, "7")
	}
}

func writeLog(t *testing.T, lines ...string) claudeLog {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "-repo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	body := strings.Join(lines, "\n") + "\n"
	path := filepath.Join(project, "session-1.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return claudeLog{root: root, sessionID: "session-1"}
}

func appendToLog(t *testing.T, log claudeLog, line string) {
	t.Helper()
	path, err := log.path()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

func assertAdditions(t *testing.T, got, want []addition) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d additions %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("addition %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}
