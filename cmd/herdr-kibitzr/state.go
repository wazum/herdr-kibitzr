package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Keyed by agent session, because that is who the counted writes belong to.
func stateFile(stateDir, session string) string {
	sum := sha256.Sum256([]byte(session))
	return filepath.Join(stateDir, hex.EncodeToString(sum[:8])+".json")
}

// Only a missing file means a session nobody has read yet. Treating anything
// else that way would throw the cursor away.
func loadState(path string) (state, error) {
	var loaded state
	if err := readJSON(path, &loaded); err != nil {
		return state{}, err
	}
	return loaded, nil
}

func saveState(path string, next state) error {
	return writeJSON(path, next)
}

func readJSON(path string, into any) error {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("unreadable state in %s: %w", path, err)
	}
	return nil
}

// This writes a sibling and renames it over the real file, so a process that
// dies mid-write leaves the old contents behind and not half of these.
func writeJSON(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, ".state-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary.Name()) }()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}
