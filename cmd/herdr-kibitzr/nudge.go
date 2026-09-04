package main

import (
	"slices"
	"strings"
)

// commit names an unpushed commit the agent made during the turn, so the fix
// goes where the slop already is instead of landing on top of it. Empty when
// nothing was committed, or when what was committed is already pushed.
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
