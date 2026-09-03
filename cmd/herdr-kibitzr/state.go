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

func stateFile(stateDir, repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return filepath.Join(stateDir, hex.EncodeToString(sum[:8])+".json")
}

// Only a file nobody has written yet means a fresh project. Anything else read
// as fresh would quietly throw away the window the nudge rule works from.
func loadState(path string) (state, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return state{}, nil
	}
	if err != nil {
		return state{}, err
	}

	var loaded state
	if err := json.Unmarshal(content, &loaded); err != nil {
		return state{}, fmt.Errorf("unreadable state in %s: %w", path, err)
	}
	return loaded, nil
}

// Written beside the real file and renamed over it, so a process that dies
// mid-write leaves the previous state rather than half of this one.
func saveState(path string, next state) error {
	content, err := json.Marshal(next)
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
