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

// Where the text came from is the caller's problem. Here it is only text.
func addedComments(added []addition) map[string][]string {
	found := map[string][]string{}
	for _, one := range added {
		if skipped(one.path) {
			continue
		}
		for _, line := range strings.Split(one.text, "\n") {
			if text := strings.TrimSpace(line); isComment(text) {
				found[one.path] = append(found[one.path], text)
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
	// A docstring opens its own line, so `sql = """` stays code.
	case strings.HasPrefix(text, `"""`), strings.HasPrefix(text, "'''"):
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

// # is a comment in Python, Ruby, PHP and shell. #include is not.
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

// A pointer dereference such as `*ptr = value` is code, not a block comment.
func continuesBlock(text string) bool {
	rest := text[1:]
	return rest == "" || strings.HasPrefix(rest, "/") ||
		strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")
}
