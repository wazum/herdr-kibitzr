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

if [[ $1 == agent && $2 == read ]]; then
	if [[ -n ${HERDR_COMPOSER_TYPED:-} ]]; then
		printf '❯ review the uncommitted\r\n'
	else
		printf '❯ \033[0m\033[2mgo build ./...\033[0m\r\n'
	fi
	exit 0
fi

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

pane=w1:p2
agent=claude

# In a file, because every call sits inside a command substitution and a
# subshell cannot hand a variable back.
statusfile="$work/last-status"
echo working > "$statusfile"

# A turn ends when an agent's status changes, so each run flips between the two
# settled statuses. Firing the same one twice is a title change, not a turn.
turn() {
	local now="done"
	if [[ $(cat "$statusfile") == "done" ]]; then now="idle"; fi
	echo "$now" > "$statusfile"
	event "$now"
}

same_status_again() {
	event "$(cat "$statusfile")"
}

event() {
	export HERDR_PLUGIN_EVENT_JSON="{\"data\":{\"pane_id\":\"$pane\",\"agent_status\":\"$1\",\"agent\":\"$agent\"}}"
	export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"$pane\",\"focused_pane_cwd\":\"$repo\",\"focused_pane_agent\":\"$agent\"}"
	./bin/herdr-kibitzr
}

# Every nudge hands the following turn to the agent. Only a turn that writes
# something spends that pass, so this stands in for the agent's reply.
spend_cleanup_pass() {
	wrote "$repo/spent.go" "// the reply to the last nudge"
	turn >/dev/null
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
	"0 comments written" "$(turn)"
refuse_prompt_calls "and prompts nobody"

# What the agent says it wrote is what gets counted.
wrote "$repo/src/Order.php" "/** @var int */\n    private int \$total;"
wrote "$repo/src/Money.php" "// a plain remark"
: > "$calls"
expect "comments the agent wrote nudge" "w1:p2 claude · nudged · 2 comments · 2 files" "$(turn)"
expect "prompt reached the pane" "agent prompt w1:p2" "$(cat "$calls")"
expect "prompt names the files" "Order.php" "$(cat "$calls")"
expect "eyes ride on the agent label" \
	"report-metadata w1:p2 --source kibitzr --display-agent claude 👀" "$(cat "$calls")"
expect "prompt file seeded" "Delete every comment" "$(cat "$HERDR_PLUGIN_CONFIG_DIR/prompt.md")"

# A title or token change repeats the status kibitzr already recorded. Acting on
# one is what dropped a prompt into a composer somebody was typing in.
wrote "$repo/src/Money.php" "// written between two turns"
: > "$calls"
expect "the same status twice is not a turn end" "still" "$(same_status_again)"
refuse_prompt_calls "and prompts nobody"

# The cleanup is itself written text, so the turn that acts on a nudge is the
# agent's own and nothing is said about it.
wrote "$repo/src/Order.php" "/** @var int */"
: > "$calls"
expect "the cleanup itself is not nudged about" "the agent's own cleanup" "$(turn)"
refuse_prompt_calls "and prompts nobody once more"

wrote "$repo/src/Money.php" "// and one after that"
expect "a comment written later still nudges" "w1:p2 claude · nudged" "$(turn)"

# A turn where the agent wrote nothing at all.
spend_cleanup_pass
: > "$calls"
expect "a turn with no writes stays quiet" "0 comments written" "$(turn)"
refuse_prompt_calls "and prompts nobody again"

# Nothing is submitted into an input somebody has typed into. Claude shows a dim
# suggestion when the line is empty, so undimmed text means a person is there.
wrote "$repo/src/Money.php" "// written while somebody was typing"
export HERDR_COMPOSER_TYPED=1
: > "$calls"
expect "a typed composer holds the nudge back" "somebody is typing" "$(turn)"
refuse_prompt_calls "and prompts nobody"
unset HERDR_COMPOSER_TYPED
expect "and the nudge arrives once the line is clear" \
	"w1:p2 claude · nudged" "$(turn)"

# A nudge the agent never received is read again next turn rather than lost.
spend_cleanup_pass
wrote "$repo/main.go" "// written while the pane was blocked"
export HERDR_REFUSE_PROMPT=1
expect "a refused prompt is reported, not recorded" "not delivered" "$(turn)"
unset HERDR_REFUSE_PROMPT
expect "the refused nudge is tried again" "w1:p2 claude · nudged · 1 comments" "$(turn)"

# A muted pane is left alone, and unmuting brings it back.
spend_cleanup_pass
wrote "$repo/main.go" "// said while muted"
export HERDR_PANE_ID=w1:p2
: > "$calls"
expect "toggling reports the mute" "muted" "$(./bin/herdr-kibitzr toggle)"
expect "a muted pane is marked in the sidebar" \
	"report-metadata w1:p2 --source kibitzr --display-agent claude 🔇" "$(cat "$calls")"

: > "$calls"
expect "a muted pane is skipped" "w1:p2 claude · quiet · muted" "$(turn)"
refuse_prompt_calls "and prompts nobody while muted"

: > "$calls"
expect "toggling back reports watching" "watching" "$(./bin/herdr-kibitzr toggle)"
expect "unmuting clears the mark" \
	"report-metadata w1:p2 --source kibitzr --clear-display-agent" "$(cat "$calls")"
expect "an unmuted pane is nudged again" "w1:p2 claude · nudged" "$(turn)"
unset HERDR_PANE_ID

# An agent whose writes cannot be read falls back to what changed on disk while
# it was working, so codex is watched without a parser for it.
pane=w1:p9
agent=codex
echo working > "$statusfile"
export HERDR_PANE_SESSION=codex-1
: > "$calls"
expect "a codex pane starts from the present" "0 comments written" "$(turn)"
refuse_prompt_calls "and prompts nobody yet"

printf '\n// written by codex just now\n' >> "$repo/notes.go"
expect "a codex pane is nudged for what changed on disk" \
	"w1:p9 codex · nudged · 1 comments" "$(turn)"

pane=w1:p2
agent=claude
expect "a working agent produces nothing" "" "$(event working)"

outside="$work/plain"
mkdir -p "$outside"
export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p2","agent_status":"done","agent":"claude"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p2\",\"focused_pane_cwd\":\"$outside\"}"
expect "a directory outside git stays quiet" "not a git repository" "$(./bin/herdr-kibitzr)"

echo
echo "smoke passed"
