// Command kibitzr watches herdr agent panes and asks an agent that just added
// code comments to review them.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const defaultPrompt = `You added comments in your last changes, listed below by file. Review each one.

Delete every comment that only restates what the code already says, including
single-line ones. Shorten what remains. Keep type annotations and docblocks the
tooling needs, and keep a comment that records why something is the way it is.

This is mechanical. Do not analyse, just read each comment and edit. Change no
code. Some files may hold no comments of yours at all, in which case say so and
leave them alone.`

const badgeTTL = 10 * time.Second

const herdrTimeout = 15 * time.Second

func main() {
	action := run
	if len(os.Args) > 1 && os.Args[1] == "toggle" {
		action = toggle
	}
	if err := action(); err != nil {
		fmt.Fprintln(os.Stderr, "kibitzr:", err)
		os.Exit(1)
	}
}

// Per pane, so one pane can go quiet while its siblings in the same repo carry
// on being watched.
func toggle() error {
	stateDir, err := ensureStateDir()
	if err != nil {
		return err
	}
	paneID := os.Getenv("HERDR_PANE_ID")
	if paneID == "" {
		return errors.New("no pane to toggle")
	}

	nowMuted, err := toggleMute(muteFile(stateDir), paneID)
	if err != nil {
		return err
	}

	word := "watching"
	if nowMuted {
		word = "muted"
	}
	fmt.Printf("%s · %s\n", paneID, word)

	// This goes in the sidebar and not into a notification, because herdr ships
	// with toast delivery off. No TTL, so the mark stands until you unmute.
	agent := focusedAgent(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
	if agent == "" {
		return nil
	}
	if nowMuted {
		return herdr("pane", "report-metadata", paneID,
			"--source", "kibitzr", "--display-agent", agent+" 🔇")
	}
	return herdr("pane", "report-metadata", paneID,
		"--source", "kibitzr", "--clear-display-agent")
}

func run() error {
	parsed, ok := turnEnd(os.Getenv("HERDR_PLUGIN_EVENT_JSON"), os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
	if !ok {
		return nil
	}
	finished := &parsed

	repo, err := topLevel(finished.cwd)
	if err != nil {
		say(finished, "quiet · not a git repository")
		return nil
	}

	stateDir, err := ensureStateDir()
	if err != nil {
		return err
	}
	answer, err := herdrOutput("pane", "get", finished.paneID)
	if err != nil {
		return err
	}
	pane := readPane(answer)

	if pane.session == "" {
		say(finished, "quiet · no agent session to read")
		return nil
	}
	finished.session = pane.session

	path := stateFile(stateDir, finished.session)

	release, locked := acquire(path + ".lock")
	if !locked {
		say(finished, "quiet · lock held")
		return nil
	}
	defer release()

	previous, err := loadState(path)
	if err != nil {
		// Start over. Staying silent until somebody deletes the file is worse.
		say(finished, "state discarded · %v", err)
		previous = state{}
	}

	if !settled(previous, finished.status) {
		say(finished, "quiet · still %s", finished.status)
		return nil
	}

	// A repository with no commit yet is fine. It just has nothing to amend.
	currentHead, _ := head(repo)

	// Every settled turn records its status, whatever else it does, or the next
	// one cannot tell a turn end from a title change.
	if muted(muteFile(stateDir), finished.paneID) {
		say(finished, "quiet · muted")
		return saveState(path, record(previous, finished.status, currentHead))
	}

	added, cursor, err := authorshipFor(finished, repo).additions(previous.Cursor)
	if err != nil {
		say(finished, "quiet · cannot read what %s wrote: %v", finished.agent, err)
		return saveState(path, record(previous, finished.status, currentHead))
	}
	comments, count := countAdded(added)

	nudge, next := decide(previous, cursor, count)
	if !nudge {
		if previous.AwaitingCleanup && count > 0 {
			say(finished, "quiet · %d comments · the agent's own cleanup", count)
		} else {
			say(finished, "quiet · %d comments written", count)
		}
		return saveState(path, record(next, finished.status, currentHead))
	}

	// The cursor stays put on both paths below, so the next turn reads the same
	// writes and tries again.
	if composerFor(finished).busy() {
		say(finished, "quiet · %d comments · somebody is typing", count)
		return saveState(path, record(previous, finished.status, currentHead))
	}

	if err := deliver(finished, comments, amendTarget(previous, currentHead, repo)); err != nil {
		say(finished, "quiet · %d comments · not delivered: %v", count, err)
		return saveState(path, record(previous, finished.status, currentHead))
	}

	say(finished, "nudged · %d comments · %d files", count, len(comments))
	return saveState(path, record(next, finished.status, currentHead))
}

// Every pane on the server writes to one plugin log, so every line has to say
// which pane it came from.
func say(finished *turn, format string, args ...any) {
	fmt.Printf("%s %s · %s\n", finished.paneID, finished.agent, fmt.Sprintf(format, args...))
}

func ensureStateDir() (string, error) {
	stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	if stateDir == "" {
		return "", errors.New("HERDR_PLUGIN_STATE_DIR is not set")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", err
	}
	return stateDir, nil
}

func countAdded(added []addition) (comments map[string][]string, count int) {
	comments = addedComments(added)
	for _, lines := range comments {
		count += len(lines)
	}
	return comments, count
}

func deliver(finished *turn, comments map[string][]string, amend string) error {
	prompt, err := promptText()
	if err != nil {
		return err
	}
	if err := herdr("agent", "prompt", finished.paneID, nudgeText(prompt, comments, amend)); err != nil {
		return err
	}
	markLooked(finished)
	return nil
}

// Only a commit the agent made during this turn, and only while no remote has
// it. Amending anything published would rewrite history somebody else may hold.
func amendTarget(previous state, currentHead, repo string) string {
	commit := committedDuring(previous, currentHead)
	if commit == "" || !amendable(repo, commit) {
		return ""
	}
	return commit[:min(len(commit), 12)]
}

// The agent label is the one piece of text a plugin can change in herdr's
// default sidebar rows. Built from the agent kind, so two nudges in a row
// cannot stack two pairs of eyes.
func markLooked(finished *turn) {
	if finished.agent == "" {
		return
	}
	// The nudge already landed, so a missing badge is not a failure.
	_ = herdr("pane", "report-metadata", finished.paneID,
		"--source", "kibitzr",
		"--display-agent", finished.agent+" 👀",
		"--token", "kibitzr=👀",
		"--ttl-ms", strconv.FormatInt(badgeTTL.Milliseconds(), 10))
}

func promptText() (string, error) {
	path := filepath.Join(os.Getenv("HERDR_PLUGIN_CONFIG_DIR"), "prompt.md")
	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(defaultPrompt+"\n"), 0o600); err != nil {
		return "", err
	}
	return defaultPrompt, nil
}

func herdr(args ...string) error {
	_, err := herdrOutput(args...)
	return err
}

func herdrOutput(args ...string) (string, error) {
	binary := os.Getenv("HERDR_BIN_PATH")
	if binary == "" {
		binary = "herdr"
	}

	ctx, cancel := context.WithTimeout(context.Background(), herdrTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", binary, args[0], err, out)
	}
	return string(out), nil
}
