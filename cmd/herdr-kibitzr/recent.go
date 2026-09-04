package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// For agents that do not record what they wrote. The cursor is an instant, and
// a file counts as this agent's work if it was written after it. Weaker than an
// agent's own account: a person typing in an editor during the same stretch is
// indistinguishable, and so is a second agent in the same repository.
type recentlyChanged struct {
	repo string
}

func (source recentlyChanged) additions(cursor string) ([]addition, string, error) {
	now := strconv.FormatInt(time.Now().UnixNano(), 10)

	// Nothing to blame anybody for on a first look, so the window opens now.
	since, parsed := parseNanos(cursor)
	if !parsed {
		return nil, now, nil
	}
	after := time.Unix(0, since)

	changed, err := source.changedFiles()
	if err != nil {
		return nil, cursor, err
	}

	var added []addition
	for path, text := range changed {
		info, err := os.Lstat(filepath.Join(source.repo, path))
		if err != nil || !info.ModTime().After(after) {
			continue
		}
		added = append(added, addition{path: path, text: text})
	}
	return added, now, nil
}

func parseNanos(cursor string) (int64, bool) {
	nanos, err := strconv.ParseInt(cursor, 10, 64)
	return nanos, err == nil
}

// What the working tree holds that the last commit does not: added lines for a
// tracked file, the whole content for one git has never seen.
func (source recentlyChanged) changedFiles() (map[string]string, error) {
	diff, err := diffFrom(source.repo, "HEAD")
	if err != nil {
		return nil, err
	}

	changed := map[string]string{}
	for path, lines := range addedByFile(diff) {
		changed[path] = strings.Join(lines, "\n")
	}

	untracked, err := untrackedFiles(source.repo)
	if err != nil {
		return nil, err
	}
	for path, content := range untracked {
		changed[path] = content
	}
	return changed, nil
}

// The added lines of a unified diff, per file.
func addedByFile(diff string) map[string][]string {
	byFile := map[string][]string{}
	path := ""
	for _, line := range strings.Split(diff, "\n") {
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			path = after
			continue
		}
		// A deletion header starts with a plus too, so without this it reads as
		// a line added to whichever file came before it.
		if line == "+++ /dev/null" {
			path = ""
			continue
		}
		added, ok := strings.CutPrefix(line, "+")
		if !ok || path == "" {
			continue
		}
		byFile[path] = append(byFile[path], added)
	}
	return byFile
}
