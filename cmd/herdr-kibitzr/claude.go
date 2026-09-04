package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// Claude Code appends one JSON object per line to a file named after the
// session, under a directory named after the project. The cursor is a byte
// offset into that file.
type claudeLog struct {
	root      string
	sessionID string
}

func (log claudeLog) additions(cursor string) ([]addition, string, error) {
	path, err := log.path()
	if err != nil {
		return nil, cursor, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, cursor, err
	}
	defer func() { _ = file.Close() }()

	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, cursor, err
	}

	// Nothing to read for a session seen for the first time.
	from, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil {
		return nil, strconv.FormatInt(end, 10), nil
	}
	// A cursor past the end, left by a log that rolled over, needs no guard of
	// its own: the limit below goes negative and reads nothing.
	if _, err := file.Seek(from, io.SeekStart); err != nil {
		return nil, cursor, err
	}

	added, err := writesIn(io.LimitReader(file, end-from))
	if err != nil {
		return nil, cursor, err
	}
	return added, strconv.FormatInt(end, 10), nil
}

func writesIn(source io.Reader) ([]addition, error) {
	var added []addition

	lines := bufio.NewScanner(source)
	lines.Buffer(make([]byte, 0, 64*1024), maxLogLineBytes)
	for lines.Scan() {
		var entry struct {
			Message struct {
				Content []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Input struct {
						Path    string `json:"file_path"`
						Content string `json:"content"`
						New     string `json:"new_string"`
						Edits   []struct {
							New string `json:"new_string"`
						} `json:"edits"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		// A line this build does not understand is no reason to stop reading.
		if json.Unmarshal(lines.Bytes(), &entry) != nil {
			continue
		}

		for _, part := range entry.Message.Content {
			if part.Type != "tool_use" {
				continue
			}
			input := part.Input
			switch part.Name {
			case "Write":
				added = append(added, addition{path: input.Path, text: input.Content})
			case "Edit":
				added = append(added, addition{path: input.Path, text: input.New})
			case "MultiEdit":
				for _, edit := range input.Edits {
					added = append(added, addition{path: input.Path, text: edit.New})
				}
			}
		}
	}
	if err := lines.Err(); err != nil {
		return nil, err
	}
	return added, nil
}

// The directory the file sits under is derived from a path we do not have, so
// it is searched for rather than computed.
func (log claudeLog) path() (string, error) {
	root := log.root
	if root == "" {
		root = os.Getenv("KIBITZR_CLAUDE_PROJECTS")
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".claude", "projects")
	}

	matches, err := filepath.Glob(filepath.Join(root, "*", log.sessionID+".jsonl"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no session log for %s under %s", log.sessionID, root)
	}
	return matches[0], nil
}
