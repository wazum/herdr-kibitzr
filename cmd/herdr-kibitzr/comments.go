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

func isComment(text string) bool {
	switch {
	case strings.HasPrefix(text, "//"), strings.HasPrefix(text, "/*"),
		strings.HasPrefix(text, "<!--"):
		return true
	case strings.HasPrefix(text, "#"):
		return !strings.HasPrefix(text, "#!") && !directive(text)
	case strings.HasPrefix(text, "*"):
		return continuesBlock(text)
	}
	return false
}

var directives = map[string]bool{
	"include": true, "define": true, "undef": true, "if": true, "ifdef": true,
	"ifndef": true, "elif": true, "elifdef": true, "elifndef": true,
	"else": true, "endif": true, "pragma": true, "error": true,
	"warning": true, "line": true, "embed": true,
}

// #include is not a comment, but # on its own line in Python, Ruby, PHP or a
// shell script is, with or without a space after it.
func directive(text string) bool {
	word := strings.TrimLeft(text[1:], " \t")
	end := strings.IndexFunc(word, func(r rune) bool {
		return r < 'a' || r > 'z'
	})
	if end >= 0 {
		word = word[:end]
	}
	return directives[word]
}

// A block comment's inner lines are `* text`, `*/` or a bare `*`. Dereferencing
// a pointer, as in `*ptr = value`, is code.
func continuesBlock(text string) bool {
	rest := text[1:]
	return rest == "" || strings.HasPrefix(rest, "/") ||
		strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")
}
