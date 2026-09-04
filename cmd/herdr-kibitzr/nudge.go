package main

import (
	"slices"
	"strings"
)

// commit is empty unless there is one the agent can still amend.
func nudgeText(prompt string, comments map[string][]string, commit string) string {
	if len(comments) == 0 {
		return prompt
	}

	files := make([]string, 0, len(comments))
	for path := range comments {
		files = append(files, path)
	}
	slices.Sort(files)

	text := prompt + "\n\n" + strings.Join(files, "\n")
	if commit != "" {
		text += "\n\nYou committed " + commit + " during that turn and it is not" +
			" pushed, so amend it rather than leaving the fix uncommitted."
	}
	return text
}
