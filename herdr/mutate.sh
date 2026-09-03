#!/usr/bin/env bash
# Mutation check for the domain logic. Each mutation is a deliberate break; the
# test suite must fail for every one of them. A surviving mutation is a gap.
# Runs against a copy, so the working tree is never modified.
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1
source_tree="$PWD"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp -R "$source_tree" "$work/repo"
cd "$work/repo" || exit 1
rm -rf bin .git

mutations=(
	"comments.go|||s/strings\.HasPrefix\(text, \"#!\"\)/false/|||shebang guard never fires"
	"comments.go|||s/return false\n\t\}\n\tfor _, marker/return true\n\t}\n\tfor _, marker/|||shebang guard inverted"
	"comments.go|||s/\|\| skipped\(path\)//|||skipped extensions counted in diffs"
	"comments.go|||s/if skipped\(path\) \{\n\t\t\tcontinue\n\t\t\}//|||skipped extensions counted in untracked files"
	"comments.go|||s/\"\/\/\", \"#\"/\"\/\/\"/|||hash marker dropped"
	"comments.go|||s/, \"<!--\"//|||markup marker dropped"
	"comments.go|||s/, \"\/\*\"//|||block open marker dropped"
	"comments.go|||s/markers = \[\]string\{\"\/\/\", \"#\", \"\/\*\", \"\*\"/markers = []string{\"\/\/\", \"#\", \"\/\*\"/|||block interior marker dropped"
	"comments.go|||s/\"\+\+\+ b\/\"/\"--- a\/\"/|||reads the pre-image filename"
	"comments.go|||s/path == \"\" \|\| //|||added lines counted before any filename"
	"comments.go|||s/strings\.TrimSpace\(added\)/added/|||indented comments missed"
	"comments.go|||s/strings\.TrimSpace\(line\)/line/|||indented comments missed in untracked files"
	"decide.go|||s/count > prev\.NudgedAtCount/count >= prev.NudgedAtCount/|||nudges when the count merely holds"
	"decide.go|||s/count > prev\.NudgedAtCount/count > 0/|||high-water mark ignored"
	"decide.go|||s/prev\.Baseline == \"\"/prev.Baseline != \"\"/|||first-run check inverted"
	"decide.go|||s/prev\.Baseline != head/prev.Baseline == head/|||baseline-moved check inverted"
	"decide.go|||s/if prev\.Baseline != head \{\n\t\treturn state\{Baseline: head\}\n\t\}//|||mark not reset after a commit"
	"decide.go|||s/return true, state\{Baseline: head, NudgedAtCount: count\}/return true, state{Baseline: prev.Baseline, NudgedAtCount: count}/|||baseline never advances on a nudge"
	"decide.go|||s/return false, prev\.advance\(head\)/return false, prev/|||baseline never advances when quiet"
	"nudge.go|||s/len\(comments\) == 0/len(comments) != 0/|||empty-file-list check inverted"
	"nudge.go|||s/\tslices\.Sort\(files\)\n//|||file list left in map order"
	"turn.go|||s/!= \"done\" \&\& event\.Data\.AgentStatus != \"idle\"/!= \"done\" || event.Data.AgentStatus != \"idle\"/|||every status treated as a turn end"
	"turn.go|||s/ \&\& event\.Data\.AgentStatus != \"idle\"//|||idle no longer a turn end"
	"turn.go|||s/context\.FocusedPaneID != event\.Data\.PaneID/context.FocusedPaneID == event.Data.PaneID/|||pane match inverted"
	"turn.go|||s/if context\.FocusedPaneID != event\.Data\.PaneID \{\n\t\treturn turn\{\}, false\n\t\}//|||pane match guard removed"
	"turn.go|||s/ \|\| context\.FocusedPaneCwd == \"\"//|||missing working directory accepted"
)

survivors=0
for entry in "${mutations[@]}"; do
	file="cmd/herdr-kibitzr/${entry%%|||*}"
	rest="${entry#*|||}"
	expression="${rest%%|||*}"
	description="${rest#*|||}"

	cp "$source_tree/$file" "$file"
	perl -0777 -pi -e "$expression" "$file"

	if git --no-pager diff --no-index --quiet "$source_tree/$file" "$file" 2>/dev/null; then
		echo "NOT APPLIED  $description"
		survivors=$((survivors + 1))
		continue
	fi

	if go test ./cmd/herdr-kibitzr/ >/dev/null 2>&1; then
		echo "SURVIVED     $description"
		survivors=$((survivors + 1))
	else
		echo "caught       $description"
	fi
done

cp "$source_tree"/cmd/herdr-kibitzr/*.go cmd/herdr-kibitzr/
echo
echo "${#mutations[@]} mutations, $survivors uncaught"
[[ $survivors -eq 0 ]]
