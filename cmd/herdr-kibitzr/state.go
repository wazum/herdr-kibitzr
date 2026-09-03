package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

func stateFile(stateDir, repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return filepath.Join(stateDir, hex.EncodeToString(sum[:8])+".json")
}

func loadState(path string) state {
	content, err := os.ReadFile(path)
	if err != nil {
		return state{}
	}
	var loaded state
	if err := json.Unmarshal(content, &loaded); err != nil {
		return state{}
	}
	return loaded
}

func saveState(path string, next state) error {
	content, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}
