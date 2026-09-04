#!/usr/bin/env bash
# End-to-end check of the hook against a real git repository and a real Claude
# session log, with a stand-in herdr binary that records what it was asked to do.
set -euo pipefail

cd "$(dirname "$0")/.."
go build -o bin/herdr-kibitzr ./cmd/herdr-kibitzr

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

repo="$work/project"
mkdir -p "$repo"
git -C "$repo" init -q --initial-branch=main
printf 'package main\n\nfunc main() {}\n' > "$repo/main.go"
git -C "$repo" add -A
git -C "$repo" -c user.email=t@example.com -c user.name=t commit -q -m first

# Claude's own account of what it wrote. One JSON object per line, exactly as
# Claude Code appends them.
log="$work/claude/-project/session-1.jsonl"
mkdir -p "$(dirname "$log")"
: > "$log"

wrote() {
	local path="$1" text="$2"
	python3 -c '
import json,sys
print(json.dumps({"type":"assistant","message":{"content":[
  {"type":"tool_use","name":"Edit","input":{"file_path":sys.argv[1],"new_string":sys.argv[2]}}]}}))
' "$path" "$text" >> "$log"
}

calls="$work/herdr-calls.log"
: > "$calls"
cat > "$work/herdr" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$HERDR_CALL_LOG"

if [[ $1 == pane && $2 == get ]]; then
	printf '{"result":{"pane":{"pane_id":"%s","focused":%s,"agent_session":{"value":"%s"}}}}\n' \
		"$3" "${HERDR_PANE_FOCUSED:-false}" "${HERDR_PANE_SESSION:-session-1}"
	exit 0
fi

# Stands in for a pane that turns out to be blocked, which herdr refuses to
# type into.
if [[ -n ${HERDR_REFUSE_PROMPT:-} && $2 == prompt ]]; then
	echo "agent_blocked" >&2
	exit 1
fi
FAKE
chmod +x "$work/herdr"

export HERDR_CALL_LOG="$calls"
export HERDR_BIN_PATH="$work/herdr"
export HERDR_PLUGIN_STATE_DIR="$work/state"
export HERDR_PLUGIN_CONFIG_DIR="$work/config"
export KIBITZR_CLAUDE_PROJECTS="$work/claude"
export HERDR_PLUGIN_EVENT_JSON='{"event":"pane_agent_status_changed","data":{"type":"pane_agent_status_changed","pane_id":"w1:p2","workspace_id":"w1","agent_status":"done","agent":"claude"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p2\",\"focused_pane_cwd\":\"$repo\",\"focused_pane_agent\":\"claude\"}"

# Every nudge hands the following turn to the agent. A scenario that wants to be
# nudged has to let that turn go by first.
spend_cleanup_pass() {
	./bin/herdr-kibitzr >/dev/null
}

# Reading a pane also goes through herdr, so an empty call log is no test of
# silence. Only a submitted prompt counts.
refuse_prompt_calls() {
	local label="$1"
	if grep -q "agent prompt" "$calls"; then
		echo "FAIL  $label"
		echo "      the agent was prompted:"
		sed 's/^/        /' "$calls"
		exit 1
	fi
	echo "ok    $label"
}

expect() {
	local label="$1" pattern="$2" actual="$3"
	if [[ $actual == *"$pattern"* ]]; then
		echo "ok    $label"
	else
		echo "FAIL  $label"
		echo "      wanted: $pattern"
		echo "      actual: $actual"
		exit 1
	fi
}

# The incident this design exists for: somebody else's comments are sitting in
# the tree when an agent arrives, and the agent is not blamed for them.
wrote "$repo/main.go" "// written by another pane entirely"
printf '\n// written by another pane entirely\n' >> "$repo/main.go"
: > "$calls"
expect "what was written before it was watched stays unread" \
	"0 comments written" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody"

# Nothing is submitted into a pane somebody may be typing in.
export HERDR_PANE_FOCUSED=true
wrote "$repo/main.go" "// typed while the pane was focused"
: > "$calls"
expect "a focused pane is left alone" "quiet · pane is focused" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody at all"
unset HERDR_PANE_FOCUSED

# What the agent says it wrote is what gets counted.
wrote "$repo/src/Order.php" "/** @var int */\n    private int \$total;"
wrote "$repo/src/Money.php" "// a plain remark"
: > "$calls"
expect "comments the agent wrote nudge" "nudged w1:p2 · 3 comments · 3 files" "$(./bin/herdr-kibitzr)"
expect "prompt reached the pane" "agent prompt w1:p2" "$(cat "$calls")"
expect "prompt names the files" "Order.php" "$(cat "$calls")"
expect "eyes ride on the agent label" \
	"report-metadata w1:p2 --source kibitzr --display-agent claude 👀" "$(cat "$calls")"
expect "prompt file seeded" "Delete every comment" "$(cat "$HERDR_PLUGIN_CONFIG_DIR/prompt.md")"

# The cleanup is itself written text, so the turn that acts on a nudge is the
# agent's own and nothing is said about it.
wrote "$repo/src/Order.php" "/** @var int */"
: > "$calls"
expect "the cleanup itself is not nudged about" "quiet" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody once more"

wrote "$repo/src/Money.php" "// and one after that"
expect "a comment written later still nudges" "nudged w1:p2 · 1 comments" "$(./bin/herdr-kibitzr)"

# A turn where the agent wrote nothing at all.
spend_cleanup_pass
: > "$calls"
expect "a turn with no writes stays quiet" "0 comments written" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody again"

# A nudge the agent never received is read again next turn rather than lost.
wrote "$repo/main.go" "// written while the pane was blocked"
export HERDR_REFUSE_PROMPT=1
expect "a refused prompt is reported, not recorded" "not delivered" "$(./bin/herdr-kibitzr)"
unset HERDR_REFUSE_PROMPT
expect "the refused nudge is tried again" "nudged w1:p2 · 1 comments" "$(./bin/herdr-kibitzr)"

# A muted pane is left alone, and unmuting brings it back.
spend_cleanup_pass
wrote "$repo/main.go" "// said while muted"
export HERDR_PANE_ID=w1:p2
: > "$calls"
expect "toggling reports the mute" "muted" "$(./bin/herdr-kibitzr toggle)"
expect "a muted pane is marked in the sidebar" \
	"report-metadata w1:p2 --source kibitzr --display-agent claude 🔇" "$(cat "$calls")"

: > "$calls"
expect "a muted pane is skipped" "quiet · muted" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody while muted"

: > "$calls"
expect "toggling back reports watching" "watching" "$(./bin/herdr-kibitzr toggle)"
expect "unmuting clears the mark" \
	"report-metadata w1:p2 --source kibitzr --clear-display-agent" "$(cat "$calls")"
expect "an unmuted pane is nudged again" "nudged w1:p2" "$(./bin/herdr-kibitzr)"
unset HERDR_PANE_ID

# An agent whose writes cannot be read falls back to what changed on disk while
# it was working, so codex is watched without a parser for it.
export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p9","agent_status":"done","agent":"codex"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p9\",\"focused_pane_cwd\":\"$repo\",\"focused_pane_agent\":\"codex\"}"
export HERDR_PANE_SESSION=codex-1
spend_cleanup_pass # the first sighting of this pane
: > "$calls"
expect "a codex pane starts from the present" "0 comments written" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody yet"

printf '\n// written by codex just now\n' >> "$repo/notes.go"
expect "a codex pane is nudged for what changed on disk" \
	"nudged w1:p9 · 1 comments" "$(./bin/herdr-kibitzr)"

export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p2","agent_status":"working"}}'
: > "$calls"
expect "a working agent produces nothing" "" "$(./bin/herdr-kibitzr)"
refuse_prompt_calls "and prompts nobody, working"

outside="$work/plain"
mkdir -p "$outside"
export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p2","agent_status":"done"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p2\",\"focused_pane_cwd\":\"$outside\"}"
expect "a directory outside git stays quiet" "not a git repository" "$(./bin/herdr-kibitzr)"

echo
echo "smoke passed"
