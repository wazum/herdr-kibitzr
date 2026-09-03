package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxFileBytes      = 1 << 20
	maxUntrackedFiles = 300
)

// A hung git call would hold the lock and one of herdr's 32 plugin slots.
const gitTimeout = 30 * time.Second

func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// quotePath off so a path outside ASCII arrives as itself rather than as
	// escapes the diff header parser would not recognise.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.quotePath=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func topLevel(dir string) (string, error) {
	return git(dir, "rev-parse", "--show-toplevel")
}

func head(dir string) (string, error) {
	return git(dir, "rev-parse", "HEAD")
}

// The three --no flags keep the output machine-readable. A repository that
// configures colour, an external diff driver or a textconv filter would
// otherwise reshape the header lines the parser depends on.
func diffFrom(dir, ref string) (string, error) {
	return git(dir, "diff", "--unified=0",
		"--no-color", "--no-ext-diff", "--no-textconv", ref)
}

// One scan serves every diff base, because what git does not track does not
// depend on which commit a diff starts from.
func untrackedFiles(dir string) (map[string]string, error) {
	// -z because a filename may contain a newline.
	listed, err := git(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}

	files := map[string]string{}
	for _, name := range strings.Split(listed, "\x00") {
		if name == "" || skipped(name) {
			continue
		}
		if len(files) >= maxUntrackedFiles {
			break
		}
		if content, ok := readSource(filepath.Join(dir, name)); ok {
			files[name] = content
		}
	}
	return files, nil
}

// Lstat rather than Stat, so a symlink is skipped instead of followed, and the
// limit is imposed while reading rather than trusted from the size beforehand.
func readSource(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return "", false
	}

	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil || len(content) > maxFileBytes {
		return "", false
	}
	return string(content), true
}
