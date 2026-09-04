package main

import (
	"regexp"
	"strings"
	"unicode"
)

// Reports whether somebody has typed into a pane's input and not sent it yet.
// A prompt submitted then arrives glued onto their half-written sentence.
//
// Screens are agent-specific and change between versions, so an adapter that
// cannot find what it is looking for answers false. Being nudged mid-sentence
// now and again is better than never being nudged at all.
type composer interface {
	busy() bool
}

func composerFor(finished *turn) composer {
	if finished.agent == "claude" {
		return claudeComposer{paneID: finished.paneID}
	}
	return unknownComposer{}
}

type unknownComposer struct{}

func (unknownComposer) busy() bool { return false }

// Claude Code marks its input with a chevron and shows a dim suggestion when
// the line is empty, so the styling is what separates the two.
type claudeComposer struct {
	paneID string
}

func (c claudeComposer) busy() bool {
	// The visible screen, because the detection source arrives with its styling
	// already stripped. Reading it moves nothing.
	screen, err := herdrOutput("agent", "read", c.paneID,
		"--source", "visible", "--format", "ansi", "--lines", "12")
	if err != nil {
		return false
	}
	return typedInto(screen)
}

const claudePromptMarker = "❯"

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func typedInto(screen string) bool {
	line, found := lastPromptLine(screen)
	if !found {
		return false
	}
	return strings.TrimSpace(undimmed(line)) != ""
}

func lastPromptLine(screen string) (string, bool) {
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if _, rest, found := strings.Cut(lines[i], claudePromptMarker); found {
			return rest, true
		}
	}
	return "", false
}

// Everything outside a dim span. Claude dims the part of a suggestion it is
// offering, so what is left is what a person put there.
func undimmed(line string) string {
	var kept strings.Builder
	dim := false

	for line != "" {
		match := sgr.FindStringIndex(line)
		if match == nil {
			if !dim {
				kept.WriteString(line)
			}
			break
		}
		if !dim {
			kept.WriteString(line[:match[0]])
		}
		switch line[match[0]:match[1]] {
		case "\x1b[2m":
			dim = true
		case "\x1b[0m", "\x1b[22m":
			dim = false
		}
		line = line[match[1]:]
	}

	// Claude pads after the marker with a non-breaking space, which TrimSpace
	// leaves alone.
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, kept.String())
}
