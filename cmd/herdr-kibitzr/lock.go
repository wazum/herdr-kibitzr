package main

import (
	"os"
	"time"
)

const staleLockAge = time.Minute

// One turn end can arrive as several events, each in its own process. Only one
// of them looks at a repository.
func acquire(path string) (func(), bool) {
	release := func() { _ = os.Remove(path) }

	if takeLock(path) {
		return release, true
	}

	// Without this, a process killed while holding the lock would silence the
	// repository for good.
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) < staleLockAge {
		return nil, false
	}
	if err := os.Remove(path); err != nil {
		return nil, false
	}
	if takeLock(path) {
		return release, true
	}
	return nil, false
}

func takeLock(path string) bool {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}
