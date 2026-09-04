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

const defaultPrompt = `You added comments in your last changes, listed below by file. Review each one. Delete every comment that only restates what the code already says, including single-line ones. Shorten what remains. Keep type annotations and docblocks the tooling needs. Do not change code.`

// A blink, not a state.
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

// Per pane, so one pane can be quiet while its siblings in the same repository
// are watched.
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

	// The sidebar rather than a notification, because herdr ships with toast
	// delivery off. No TTL: the mark stands until the pane is unmuted.
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
	finished, ok := turnEnd(os.Getenv("HERDR_PLUGIN_EVENT_JSON"), os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
	if !ok {
		return nil
	}

	repo, err := topLevel(finished.cwd)
	if err != nil {
		fmt.Println("quiet · not a git repository")
		return nil
	}

	stateDir, err := ensureStateDir()
	if err != nil {
		return err
	}
	if muted(muteFile(stateDir), finished.paneID) {
		fmt.Println("quiet · muted")
		return nil
	}

	answer, err := herdrOutput("pane", "get", finished.paneID)
	if err != nil {
		return err
	}
	pane := readPane(answer)

	// A prompt submitted into a focused pane arrives glued onto whatever the
	// person was already typing.
	if pane.focused {
		fmt.Println("quiet · pane is focused")
		return nil
	}
	if pane.session == "" {
		fmt.Println("quiet · no agent session to read")
		return nil
	}
	finished.session = pane.session

	path := stateFile(stateDir, finished.session)

	release, locked := acquire(path + ".lock")
	if !locked {
		fmt.Println("quiet · lock held")
		return nil
	}
	defer release()

	previous, err := loadState(path)
	if err != nil {
		// Start over rather than stay silent until somebody deletes the file.
		fmt.Printf("state discarded · %v\n", err)
		previous = state{}
	}

	added, cursor, err := authorshipFor(finished, repo).additions(previous.Cursor)
	if err != nil {
		fmt.Printf("quiet · cannot read what %s wrote: %v\n", finished.agent, err)
		return nil
	}
	comments, count := countAdded(added)

	nudge, next := decide(previous, cursor, count)
	if !nudge {
		fmt.Printf("quiet · %d comments written\n", count)
		return saveState(path, next)
	}

	// Nothing recorded, so the next turn reads the same writes again and retries.
	if err := deliver(finished, comments); err != nil {
		fmt.Printf("quiet · %d comments · not delivered: %v\n", count, err)
		return nil
	}

	fmt.Printf("nudged %s · %d comments · %d files\n", finished.paneID, count, len(comments))
	return saveState(path, next)
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

func deliver(finished turn, comments map[string][]string) error {
	prompt, err := promptText()
	if err != nil {
		return err
	}
	if err := herdr("agent", "prompt", finished.paneID, nudgeText(prompt, comments)); err != nil {
		return err
	}
	markLooked(finished)
	return nil
}

// The agent label is the one piece of text in herdr's default sidebar rows a
// plugin can change. Built from the agent kind, so two nudges cannot stack two
// pairs of eyes.
func markLooked(finished turn) {
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
