package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRecentlyChangedIgnoresWhatWasAlreadyThere(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "old.go", "package main\n// written long ago\n")
	commit(t, repo)
	write(t, repo, "old.go", "package main\n// written long ago\n// and edited long ago\n")
	backdate(t, repo, "old.go", time.Now().Add(-time.Hour))

	source := recentlyChanged{repo: repo}
	added, next, err := source.additions(cursorAt(time.Now().Add(-time.Minute)))
	if err != nil {
		t.Fatalf("additions: %v", err)
	}

	if len(added) != 0 {
		t.Errorf("reported %v, all of it older than the cursor", added)
	}
	if next == "" {
		t.Error("no cursor to resume from")
	}
}

func TestRecentlyChangedReportsWhatMovedAfterTheCursor(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "kept.go", "package main\n")
	write(t, repo, "stale.go", "package main\n")
	commit(t, repo)

	cursor := cursorAt(time.Now().Add(-time.Second))
	write(t, repo, "stale.go", "package main\n// stale edit\n")
	backdate(t, repo, "stale.go", time.Now().Add(-time.Hour))
	write(t, repo, "kept.go", "package main\n// a fresh remark\n")
	write(t, repo, "brand-new.go", "// a whole new file\n")

	added, _, err := recentlyChanged{repo: repo}.additions(cursor)
	if err != nil {
		t.Fatalf("additions: %v", err)
	}

	paths := map[string]bool{}
	for _, one := range added {
		paths[filepath.Base(one.path)] = true
	}
	if !paths["kept.go"] {
		t.Errorf("missed the file edited after the cursor, saw %v", paths)
	}
	if !paths["brand-new.go"] {
		t.Errorf("missed the new file, saw %v", paths)
	}
	if paths["stale.go"] {
		t.Errorf("reported a file whose edit predates the cursor, saw %v", paths)
	}
}

// Nothing has been watched yet, so nothing is anybody's fault.
func TestRecentlyChangedStartsAtThePresent(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	commit(t, repo)
	write(t, repo, "main.go", "package main\n// already here\n")

	added, next, err := recentlyChanged{repo: repo}.additions("")
	if err != nil {
		t.Fatalf("additions: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("reported %v on a first look", added)
	}
	if next == "" {
		t.Error("no cursor to resume from")
	}
}

func TestAddedByFileReadsOnlyAddedLinesUnderTheirOwnFile(t *testing.T) {
	diff := "+ stray line before any file header\n" +
		"+++ b/main.go\n" +
		"@@ -1,0 +1,2 @@\n" +
		"+// kept\n" +
		"-// removed\n" +
		" unchanged\n" +
		"+++ b/other.go\n" +
		"+// also kept\n"

	got := addedByFile(diff)

	if len(got) != 2 {
		t.Fatalf("got %v, want two files", got)
	}
	if len(got["main.go"]) != 1 || got["main.go"][0] != "// kept" {
		t.Errorf("main.go: got %v", got["main.go"])
	}
	if len(got["other.go"]) != 1 || got["other.go"][0] != "// also kept" {
		t.Errorf("other.go: got %v", got["other.go"])
	}
}

func cursorAt(when time.Time) string {
	return strconv.FormatInt(when.UnixNano(), 10)
}

func backdate(t *testing.T, repo, name string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(repo, name), when, when); err != nil {
		t.Fatal(err)
	}
}
