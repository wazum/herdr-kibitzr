#!/usr/bin/env bash
# End-to-end check of the hook against a real git repository, with a stand-in
# herdr binary that records what it was asked to do.
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

calls="$work/herdr-calls.log"
: > "$calls"
cat > "$work/herdr" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$HERDR_CALL_LOG"
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
export HERDR_PLUGIN_EVENT_JSON='{"event":"pane_agent_status_changed","data":{"type":"pane_agent_status_changed","pane_id":"w1:p2","workspace_id":"w1","agent_status":"done","agent":"claude"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p2\",\"focused_pane_cwd\":\"$repo\"}"

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

first="$(./bin/herdr-kibitzr)"
expect "first turn end records a baseline" "baseline recorded" "$first"
[[ -s $calls ]] && {
	echo "FAIL  first turn end prompted the agent"
	exit 1
}

printf 'package main\n\n// adds two numbers together\nfunc add(a, b int) int { return a + b }\n' >> "$repo/main.go"
printf '// a brand new file\npackage extra\n' > "$repo/extra.go"

second="$(./bin/herdr-kibitzr)"
expect "added comments nudge" "nudged w1:p2 · 2 comments · 2 files" "$second"
expect "prompt reached the pane" "agent prompt w1:p2" "$(cat "$calls")"
expect "prompt names the files" "extra.go" "$(cat "$calls")"
expect "eyes badge reported" "report-metadata w1:p2 --source kibitzr" "$(cat "$calls")"
expect "prompt file seeded" "Delete every comment" "$(cat "$HERDR_PLUGIN_CONFIG_DIR/prompt.md")"

: > "$calls"
third="$(./bin/herdr-kibitzr)"
expect "an unchanged tree stays quiet" "quiet · 2 comments · mark 2" "$third"
[[ -s $calls ]] && {
	echo "FAIL  quiet turn end still prompted the agent"
	exit 1
}

# Committing does not re-nudge: the count still comes from the old baseline, and
# those comments were already spoken for.
git -C "$repo" add -A
git -C "$repo" -c user.email=t@example.com -c user.name=t commit -q -m second
expect "a commit does not re-nudge" "quiet · 2 comments · mark 2" "$(./bin/herdr-kibitzr)"

# The baseline has moved, so what the commit contains is now settled.
expect "the committed comments are behind the baseline" "quiet · 0 comments · mark 0" "$(./bin/herdr-kibitzr)"

printf '\n// one more remark\n' >> "$repo/main.go"
expect "a comment after the commit nudges again" "nudged w1:p2 · 1 comments" "$(./bin/herdr-kibitzr)"

# A turn that adds comments and commits them in one go. The mark counted here
# comes from the old baseline, so storing it against the new one would swallow
# every later comment until the count climbed past it again.
printf '\n// three\n// more\n// remarks\n' >> "$repo/main.go"
git -C "$repo" add -A
git -C "$repo" -c user.email=t@example.com -c user.name=t commit -q -m third
expect "a turn that adds and commits nudges" "nudged w1:p2 · 4 comments" "$(./bin/herdr-kibitzr)"

printf '\n// and one after that\n' >> "$repo/main.go"
expect "the next comment is still noticed" "nudged w1:p2 · 1 comments" "$(./bin/herdr-kibitzr)"

# A nudge the agent never received, on a turn that also committed. Both facts
# together are what used to record the comments as covered and lose them.
printf '\n// written while the pane was blocked\n' >> "$repo/main.go"
git -C "$repo" add -A
git -C "$repo" -c user.email=t@example.com -c user.name=t commit -q -m fourth
export HERDR_REFUSE_PROMPT=1
expect "a refused prompt is reported, not recorded" "not delivered" "$(./bin/herdr-kibitzr)"
unset HERDR_REFUSE_PROMPT
# Two, because the baseline never moved: the comment from the turn before is
# still in range alongside the one written while the pane was blocked.
expect "the refused nudge is tried again" "nudged w1:p2 · 2 comments" "$(./bin/herdr-kibitzr)"

export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p2","agent_status":"working"}}'
: > "$calls"
working="$(./bin/herdr-kibitzr)"
expect "a working agent produces nothing" "" "$working"
[[ -s $calls ]] && {
	echo "FAIL  a working agent was prompted"
	exit 1
}

outside="$work/plain"
mkdir -p "$outside"
export HERDR_PLUGIN_EVENT_JSON='{"data":{"pane_id":"w1:p2","agent_status":"done"}}'
export HERDR_PLUGIN_CONTEXT_JSON="{\"focused_pane_id\":\"w1:p2\",\"focused_pane_cwd\":\"$outside\"}"
expect "a directory outside git stays quiet" "not a git repository" "$(./bin/herdr-kibitzr)"

echo
echo "smoke passed"
