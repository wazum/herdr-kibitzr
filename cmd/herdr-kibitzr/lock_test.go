package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The protected work can outlast any age a bystander might call stale: several
// git calls, then two herdr calls, each under its own timeout. A holder that is
// simply slow must keep its lock.
func TestAcquireKeepsTheLockWhileTheHolderIsSlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.lock")

	release, held := acquire(path)
	if !held {
		t.Fatal("could not take a free lock")
	}
	defer release()

	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	if _, second := acquire(path); second {
		t.Error("took the lock from a holder that was still running")
	}
}

func TestAcquireExcludesASecondHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.lock")

	release, held := acquire(path)
	if !held {
		t.Fatal("could not take a free lock")
	}

	if _, second := acquire(path); second {
		t.Error("took a lock somebody else was holding")
	}

	release()

	release2, again := acquire(path)
	if !again {
		t.Fatal("could not take the lock after it was released")
	}
	release2()
}
