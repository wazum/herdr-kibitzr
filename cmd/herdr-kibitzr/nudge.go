package main

import (
	"slices"
	"strings"
)

func nudgeText(prompt string, comments map[string][]string) string {
	if len(comments) == 0 {
		return prompt
	}
	files := make([]string, 0, len(comments))
	for path := range comments {
		files = append(files, path)
	}
	slices.Sort(files)
	return prompt + "\n\n" + strings.Join(files, "\n")
}
