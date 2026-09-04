package main

import (
	"os"
	"syscall"
)

// One turn end can arrive as several events, each in its own process. Only one
// of them gets to look. The kernel drops this lock however the process exits,
// so nothing has to guess when a holder died, and releasing it cannot take a
// lock somebody else holds.
func acquire(path string) (func(), bool) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, false
	}
	return func() { _ = file.Close() }, true
}
