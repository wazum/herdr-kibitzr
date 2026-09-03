// Command kibitzr watches herdr agent panes and asks an agent that just added
// code comments to review them.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const defaultPrompt = `You added comments in your last changes, listed below by file. Review each one. Delete every comment that only restates what the code already says, including single-line ones. Shorten what remains. Keep type annotations and docblocks the tooling needs. Do not change code.`

// The eyes replace the pane's status text in herdr's sidebar. They only show
// while the pane sits at the turn end that was nudged.
const badgeTTL = 10 * time.Minute

const herdrTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kibitzr:", err)
		os.Exit(1)
	}
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

	stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	path := stateFile(stateDir, repo)

	release, locked := acquire(path + ".lock")
	if !locked {
		fmt.Println("quiet · lock held")
		return nil
	}
	defer release()

	current, err := head(repo)
	if err != nil {
		fmt.Println("quiet · no commit to measure from")
		return nil
	}

	previous, err := loadState(path)
	if err != nil {
		// Say so and start over. Staying silent until somebody deletes the file
		// would be worse than losing one window.
		fmt.Printf("state discarded · %v\n", err)
		previous = state{}
	}
	if previous.Baseline == "" {
		fmt.Printf("baseline recorded · %s\n", short(current))
		return saveState(path, state{Baseline: current})
	}

	untracked, err := untrackedFiles(repo)
	if err != nil {
		return err
	}
	diff, err := diffFrom(repo, previous.Baseline)
	if err != nil {
		return err
	}
	comments, count := countAdded(diff, untracked)

	// A second diff only when the agent committed during this turn, so the mark
	// is stored against the baseline it will next be compared with.
	sinceHead := count
	if previous.Baseline != current {
		headDiff, headErr := diffFrom(repo, current)
		if headErr != nil {
			return headErr
		}
		_, sinceHead = countAdded(headDiff, untracked)
	}

	nudge, next := decide(previous, current, count, sinceHead)
	if !nudge {
		fmt.Printf("quiet · %d comments · mark %d\n", count, previous.NudgedAtCount)
		return saveState(path, next)
	}

	// Nothing recorded, so the next turn end tries again. Advancing the baseline
	// here would bury comments the agent committed but was never told about.
	if err := deliver(finished.paneID, comments); err != nil {
		fmt.Printf("quiet · %d comments · not delivered: %v\n", count, err)
		return nil
	}

	fmt.Printf("nudged %s · %d comments · %d files\n", finished.paneID, count, len(comments))
	return saveState(path, next)
}

func countAdded(diff string, untracked map[string]string) (comments map[string][]string, count int) {
	comments = addedComments(diff, untracked)
	for _, lines := range comments {
		count += len(lines)
	}
	return comments, count
}

func deliver(paneID string, comments map[string][]string) error {
	prompt, err := promptText()
	if err != nil {
		return err
	}
	if err := herdr("agent", "prompt", paneID, nudgeText(prompt, comments)); err != nil {
		return err
	}

	// The nudge already landed, so a missing badge is not a failure.
	_ = herdr("pane", "report-metadata", paneID,
		"--source", "kibitzr",
		"--state-label", "done=👀 done",
		"--state-label", "idle=👀 idle",
		"--ttl-ms", strconv.FormatInt(badgeTTL.Milliseconds(), 10))
	return nil
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
	binary := os.Getenv("HERDR_BIN_PATH")
	if binary == "" {
		binary = "herdr"
	}

	ctx, cancel := context.WithTimeout(context.Background(), herdrTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", binary, args[0], err, out)
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
