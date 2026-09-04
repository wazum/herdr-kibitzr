package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDiffFromReportsAddedLinesForATrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	commit(t, repo)
	write(t, repo, "main.go", "package main\n\n// added later\n")

	got := addedByFile(diffOrFail(t, repo, "HEAD"))

	if !slices.Contains(got["main.go"], "// added later") {
		t.Errorf("got %v, want the added comment among the added lines", got)
	}
}

func TestDiffFromReadsAPathThatIsNotPlainAscii(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "src/Bestellung.go", "package src\n")
	commit(t, repo)
	write(t, repo, "src/Bestellung.go", "package src\n\n// Grüße aus München\n")

	got := addedByFile(diffOrFail(t, repo, "HEAD"))

	if !slices.Contains(got["src/Bestellung.go"], "// Grüße aus München") {
		t.Errorf("got %v, want the non-ASCII path recognised", got)
	}
}

// A deletion's header is `+++ /dev/null`, and with no context lines it carries
// no added lines, so nothing can be credited to the file named before it.
func TestDiffFromCreditsNothingToTheFileBeforeADeletion(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.go", "package a\n")
	write(t, repo, "b.go", "package b\n// a comment that is about to go\n")
	commit(t, repo)

	if err := os.Remove(filepath.Join(repo, "b.go")); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.go", "package a\n// the only added line\n")

	got := addedByFile(diffOrFail(t, repo, "HEAD"))

	if !slices.Contains(got["a.go"], "// the only added line") {
		t.Errorf("a.go: got %v", got["a.go"])
	}
	if len(got) != 1 {
		t.Errorf("got %v, want a.go alone", got)
	}
	for _, line := range got["a.go"] {
		if strings.Contains(line, "/dev/null") {
			t.Errorf("a.go: the deletion header was read as content: %q", line)
		}
	}
}

// Rewriting a published commit changes history somebody else may hold.
func TestAmendableOnlyWhileACommitIsUnpushed(t *testing.T) {
	origin := newRepo(t)
	write(t, origin, "main.go", "package main\n")
	commit(t, origin)

	clone := t.TempDir()
	runGit(t, clone, "clone", "-q", origin, clone)
	pushed, err := head(clone)
	if err != nil {
		t.Fatal(err)
	}

	if amendable(clone, pushed) {
		t.Error("offered to amend a commit the remote already has")
	}

	write(t, clone, "main.go", "package main\n// local only\n")
	local := commit(t, clone)

	if !amendable(clone, local) {
		t.Error("refused to amend a commit no remote has")
	}
}

func TestAmendableSaysNoWithoutARemoteAnswer(t *testing.T) {
	if amendable(t.TempDir(), "deadbeef") {
		t.Error("offered to amend from a directory that is not a repository")
	}
}

func TestUntrackedFilesSkipsHugeFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	commit(t, repo)
	write(t, repo, "bundle.js", "// generated\n"+strings.Repeat("x", maxFileBytes))

	if _, ok := untrackedOrFail(t, repo)["bundle.js"]; ok {
		t.Error("read an untracked file past the size limit")
	}
}

func TestUntrackedFilesSkipsSymlinks(t *testing.T) {
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
func TestUntrackedFilesDoesNotSpendTheQuotaOnSkippedExtensions(t *testing.T) {
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

func TestUntrackedFilesStopsAtTheQuota(t *testing.T) {
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
