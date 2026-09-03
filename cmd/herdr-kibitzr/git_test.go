package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadsChangesSinceTheBaseline(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	baseline := commit(t, repo)
	write(t, repo, "main.go", "package main\n\n// added later\n")
	write(t, repo, "extra.go", "// brand new\n")

	got, _ := countAdded(diffOrFail(t, repo, baseline), untrackedOrFail(t, repo))

	assertComments(t, got, map[string][]string{
		"main.go":  {"// added later"},
		"extra.go": {"// brand new"},
	})
}

func TestSeesCommentsTheAgentCommittedDuringTheTurn(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	baseline := commit(t, repo)

	write(t, repo, "main.go", "package main\n\n// committed mid-turn\n")
	if commit(t, repo) == baseline {
		t.Fatal("the commit did not move HEAD")
	}

	got, _ := countAdded(diffOrFail(t, repo, baseline), untrackedOrFail(t, repo))

	assertComments(t, got, map[string][]string{"main.go": {"// committed mid-turn"}})
}

func TestReadsAPathThatIsNotPlainAscii(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "src/Bestellung.go", "package src\n")
	baseline := commit(t, repo)
	write(t, repo, "src/Bestellung.go", "package src\n\n// Grüße aus München\n")
	write(t, repo, "src/Ünïcode Datei.go", "// auch hier\n")

	got, _ := countAdded(diffOrFail(t, repo, baseline), untrackedOrFail(t, repo))

	assertComments(t, got, map[string][]string{
		"src/Bestellung.go":    {"// Grüße aus München"},
		"src/Ünïcode Datei.go": {"// auch hier"},
	})
}

// A deletion's header is `+++ /dev/null`. Left unrecognised, the following
// hunk would be credited to whichever file was named before it.
func TestDeletingAFileCreditsNothingToTheFileBefore(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.go", "package a\n")
	write(t, repo, "b.go", "package b\n// a comment that is about to go\n")
	baseline := commit(t, repo)

	if err := os.Remove(filepath.Join(repo, "b.go")); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.go", "package a\n// the only added comment\n")

	got, count := countAdded(diffOrFail(t, repo, baseline), untrackedOrFail(t, repo))

	if count != 1 {
		t.Errorf("count %d, want 1: %v", count, got)
	}
	assertComments(t, got, map[string][]string{"a.go": {"// the only added comment"}})
}

func TestSkipsHugeUntrackedFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	commit(t, repo)
	write(t, repo, "bundle.js", "// generated\n"+strings.Repeat("x", maxFileBytes))

	if _, ok := untrackedOrFail(t, repo)["bundle.js"]; ok {
		t.Error("read an untracked file past the size limit")
	}
}

func TestSkipsUntrackedSymlinks(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	commit(t, repo)
	write(t, repo, "real.go", "// a comment\n")
	if err := os.Symlink("real.go", filepath.Join(repo, "link.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	untracked := untrackedOrFail(t, repo)

	if _, ok := untracked["link.go"]; ok {
		t.Error("followed a symlink instead of skipping it")
	}
	if _, ok := untracked["real.go"]; !ok {
		t.Error("skipped the regular file too")
	}
}

// The quota exists to bound reading. Files that are discarded anyway must not
// spend it, or one generated directory hides the source beside it.
func TestSkippedExtensionsDoNotSpendTheQuota(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	commit(t, repo)

	for i := range maxUntrackedFiles + 50 {
		write(t, repo, fmt.Sprintf("data/file%d.json", i), "{}\n")
	}
	write(t, repo, "zz-last.go", "// still seen\n")

	untracked := untrackedOrFail(t, repo)

	if _, ok := untracked["zz-last.go"]; !ok {
		t.Errorf("the source file was crowded out by %d skipped files", len(untracked))
	}
}

func TestStopsReadingAfterTooManyUntrackedFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	commit(t, repo)

	// A scaffolded project before anyone wrote a .gitignore. Reading every
	// generated file would hold a plugin slot for as long as it takes.
	for i := range maxUntrackedFiles + 50 {
		write(t, repo, fmt.Sprintf("generated/file%d.go", i), "// generated\n")
	}

	untracked := untrackedOrFail(t, repo)

	if len(untracked) > maxUntrackedFiles {
		t.Errorf("read %d untracked files, want at most %d", len(untracked), maxUntrackedFiles)
	}
	if len(untracked) == 0 {
		t.Error("read nothing at all")
	}
}

func diffOrFail(t *testing.T, repo, ref string) string {
	t.Helper()
	diff, err := diffFrom(repo, ref)
	if err != nil {
		t.Fatalf("diffFrom: %v", err)
	}
	return diff
}

func untrackedOrFail(t *testing.T, repo string) map[string]string {
	t.Helper()
	untracked, err := untrackedFiles(repo)
	if err != nil {
		t.Fatalf("untrackedFiles: %v", err)
	}
	return untracked
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "--initial-branch=main")
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir string) string {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=t",
		"commit", "-q", "-m", "commit")
	sha, err := head(dir)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
