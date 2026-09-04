#!/usr/bin/env bash
# Does the prompt actually get an agent to do the right thing?
#
# Not part of `just check` or CI: it spends a real agent turn and its answer is
# not deterministic. Run it after editing prompt.md, and read the verdict as
# evidence rather than proof.
#
# It plants comments that must go and comments that must survive, hands the
# prompt to a live agent through herdr, and reports which ones it got right.
#
#   bash herdr/prompt-check.sh [--kind claude] [--keep]
set -euo pipefail

cd "$(dirname "$0")/.."

kind=claude
keep=""
parent=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		--kind) kind="$2"; shift 2 ;;
		--keep) keep=1; shift ;;
		# An agent asks to trust a directory it has not seen. Planting inside one
		# it already trusts skips that, which is what --in is for.
		--in) parent="$2"; shift 2 ;;
		*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
done

command -v herdr >/dev/null || {
	echo "herdr is not on PATH" >&2
	exit 1
}

if [[ -n $parent ]]; then
	work="$parent/prompt-check-$$"
	mkdir -p "$work"
else
	work="$(mktemp -d)"
fi
repo="$work/subject"
mkdir -p "$repo/src"
git -C "$repo" init -q --initial-branch=main

# The planted comments. The two lists at the bottom of this script say which
# ones are meant to go and which are meant to survive.
cat > "$repo/src/orders.py" <<'SUBJECT'
import shutil


class OrderTotal:
    """Compute the total of an order."""

    def __init__(self, lines: list[int]) -> None:
        # store the lines
        self.lines = lines
        # the running sum
        self.total = 0

    def compute(self) -> int:
        # loop over every line
        for line in self.lines:
            self.total += line
        # return the total
        return self.total

    def temp_dir(self) -> str:
        # shutil.rmtree follows symlinks out of the tree unless this is set,
        # which is why the flag is here and not in the caller.
        return shutil.mkdtemp()
SUBJECT

cat > "$repo/src/Money.php" <<'SUBJECT'
<?php

final class Money
{
    /** @var array<int, array{amount: int, currency: string}> */
    private array $parts = [];

    // adds an amount
    public function add(int $amount, string $currency): void
    {
        // append to the parts
        $this->parts[] = ['amount' => $amount, 'currency' => $currency];
    }

    // Rounding happens here rather than at display time because the tax
    // report has to agree with the invoice to the cent.
    public function total(): int
    {
        return (int) round(array_sum(array_column($this->parts, 'amount')));
    }
}
SUBJECT

git -C "$repo" add -A
git -C "$repo" -c user.email=t@example.com -c user.name=t commit -q -m subject

cleanup() {
	[[ -n $keep ]] && { echo; echo "kept: $repo"; return; }
	rm -rf "$work"
	[[ -n ${workspace:-} ]] && herdr workspace close "$workspace" >/dev/null 2>&1
	return 0
}
trap cleanup EXIT

echo "starting a $kind agent on a planted repository"
workspace="$(herdr workspace create --cwd "$repo" --label prompt-check 2>/dev/null |
	python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["workspace"]["workspace_id"])')"
pane="$(herdr pane list 2>/dev/null |
	python3 -c "
import json,sys
for p in json.load(sys.stdin)['result']['panes']:
    if p.get('workspace_id')=='$workspace': print(p['pane_id']); break")"

if ! herdr agent start promptcheck --kind "$kind" --pane "$pane" --timeout 90000 >/dev/null 2>&1; then
	echo "the agent is waiting for something, most likely asking to trust $repo."
	echo "approve it in the pane; this waits up to two minutes."
	herdr agent wait promptcheck --until idle --timeout 120000 >/dev/null || {
		echo "gave up waiting for the agent to become ready" >&2
		exit 1
	}
fi

prompt="$(cat "$(herdr plugin config-dir wazum.kibitzr 2>/dev/null |
	tr -d '\n')/prompt.md" 2>/dev/null || true)"
[[ -n $prompt ]] || prompt="$(./bin/herdr-kibitzr --print-prompt 2>/dev/null || true)"
[[ -n $prompt ]] || {
	echo "no prompt.md to check; run kibitzr once so it is seeded" >&2
	exit 1
}

echo "handing it the prompt under test"
herdr agent prompt promptcheck \
	"$prompt

src/orders.py
src/Money.php" --wait --until idle --until "done" --timeout 300000 >/dev/null

should_go=(
	'"""Compute the total of an order."""'
	'# store the lines'
	'# the running sum'
	'# loop over every line'
	'# return the total'
	'// adds an amount'
	'// append to the parts'
)
should_stay=(
	'@var array<int, array{amount: int, currency: string}>'
	'shutil.rmtree follows symlinks'
	'Rounding happens here rather than at display time'
)

echo
wrong=0
for comment in "${should_go[@]}"; do
	if grep -qF -- "$comment" "$repo"/src/*; then
		echo "kept, should have gone   $comment"
		wrong=$((wrong + 1))
	else
		echo "deleted                  $comment"
	fi
done
for comment in "${should_stay[@]}"; do
	if grep -qF -- "$comment" "$repo"/src/*; then
		echo "kept                     $comment"
	else
		echo "deleted, should have stayed   $comment"
		wrong=$((wrong + 1))
	fi
done

echo
if ! git -C "$repo" diff --quiet -- '*.py' '*.php'; then
	added="$(git -C "$repo" diff --unified=0 | grep -cE '^\+[^+]' || true)"
	removed="$(git -C "$repo" diff --unified=0 | grep -cE '^-[^-]' || true)"
	echo "the agent removed $removed lines and added $added"
fi

echo "$wrong of $(( ${#should_go[@]} + ${#should_stay[@]} )) comments handled wrongly"
[[ $wrong -eq 0 ]]
