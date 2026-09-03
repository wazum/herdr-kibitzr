package main

import (
	"os"
	"syscall"
)

// One turn end can arrive as several events, each in its own process. Only one
// of them looks at a repository. The kernel drops the lock however the process
// exits, so there is no age at which a holder has to be assumed dead, and a
// release cannot remove a lock somebody else is holding.
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
