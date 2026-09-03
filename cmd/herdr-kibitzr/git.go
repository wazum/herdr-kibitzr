package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxFileBytes = 1 << 20

const maxUntrackedFiles = 300

// A hung git call would hold the lock and one of herdr's 32 plugin slots.
const gitTimeout = 30 * time.Second

func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
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

func collect(dir, baseline string) (diff string, untracked map[string]string, err error) {
	diff, err = git(dir, "diff", "--unified=0", baseline)
	if err != nil {
		return "", nil, err
	}

	listed, err := git(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", nil, err
	}

	untracked = map[string]string{}
	for _, name := range strings.Split(listed, "\n") {
		if name == "" {
			continue
		}
		if len(untracked) >= maxUntrackedFiles {
			break
		}
		path := filepath.Join(dir, name)
		info, statErr := os.Stat(path)
		if statErr != nil || info.Size() > maxFileBytes {
			continue
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		untracked[name] = string(content)
	}
	return diff, untracked, nil
}
