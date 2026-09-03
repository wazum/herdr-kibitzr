package main

import (
	"path/filepath"
	"strings"
)

var skippedExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true, ".yml": true, ".yaml": true,
	".json": true, ".toml": true, ".lock": true, ".csv": true, ".svg": true,
}

func skipped(path string) bool {
	return skippedExtensions[strings.ToLower(filepath.Ext(path))]
}

func addedComments(diff string, untracked map[string]string) map[string][]string {
	found := map[string][]string{}
	path := ""
	for _, line := range strings.Split(diff, "\n") {
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			path = after
			continue
		}
		added, ok := strings.CutPrefix(line, "+")
		if !ok || path == "" || skipped(path) {
			continue
		}
		if text := strings.TrimSpace(added); isComment(text) {
			found[path] = append(found[path], text)
		}
	}
	for path, content := range untracked {
		if skipped(path) {
			continue
		}
		for _, line := range strings.Split(content, "\n") {
			if text := strings.TrimSpace(line); isComment(text) {
				found[path] = append(found[path], text)
			}
		}
	}
	return found
}

var markers = []string{"//", "#", "/*", "*", "<!--"}

func isComment(text string) bool {
	if strings.HasPrefix(text, "#!") {
		return false
	}
	for _, marker := range markers {
		if strings.HasPrefix(text, marker) {
			return true
		}
	}
	return false
}
