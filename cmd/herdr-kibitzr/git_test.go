package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectReadsChangesSinceTheBaseline(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	baseline := commit(t, repo)
	write(t, repo, "main.go", "package main\n\n// added later\n")
	write(t, repo, "extra.go", "// brand new\n")

	diff, untracked, err := collect(repo, baseline)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	got := addedComments(diff, untracked)
	assertComments(t, got, map[string][]string{
		"main.go":  {"// added later"},
		"extra.go": {"// brand new"},
	})
}

func TestCollectSeesCommentsTheAgentCommittedDuringTheTurn(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n")
	baseline := commit(t, repo)

	write(t, repo, "main.go", "package main\n\n// committed mid-turn\n")
	moved := commit(t, repo)

	if moved == baseline {
		t.Fatal("the commit did not move HEAD")
	}

	diff, untracked, err := collect(repo, baseline)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	got := addedComments(diff, untracked)
	assertComments(t, got, map[string][]string{"main.go": {"// committed mid-turn"}})
}

func TestCollectSkipsHugeUntrackedFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	baseline := commit(t, repo)
	write(t, repo, "bundle.js", "// generated\n"+strings.Repeat("x", maxFileBytes))

	_, untracked, err := collect(repo, baseline)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if _, ok := untracked["bundle.js"]; ok {
		t.Error("read an untracked file past the size limit")
	}
}

func TestCollectStopsReadingAfterTooManyUntrackedFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.go", "package main\n")
	baseline := commit(t, repo)

	// A scaffolded project before anyone wrote a .gitignore. Reading every
	// generated file would hold a plugin slot for as long as it takes.
	for i := range maxUntrackedFiles + 50 {
		write(t, repo, fmt.Sprintf("generated/file%d.go", i), "// generated\n")
	}

	_, untracked, err := collect(repo, baseline)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if len(untracked) > maxUntrackedFiles {
		t.Errorf("read %d untracked files, want at most %d", len(untracked), maxUntrackedFiles)
	}
	if len(untracked) == 0 {
		t.Error("read nothing at all")
	}
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
