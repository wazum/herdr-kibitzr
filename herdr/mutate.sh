#!/usr/bin/env bash
# Mutation check for the domain logic. Each mutation is a deliberate break; the
# test suite must fail for every one of them. A surviving mutation is a gap.
# Runs against a copy, so the working tree is never modified.
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1
source_tree="$PWD"
package="./cmd/herdr-kibitzr/"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp -R "$source_tree" "$work/repo"
cd "$work/repo" || exit 1
rm -rf bin .git

# A toolchain that cannot build or an already-red suite would make every
# mutation look caught, because this script reads a failing exit as success.
if ! build_output="$(go build "$package" 2>&1)"; then
	echo "cannot build the unmutated copy, so nothing below would mean anything:" >&2
	echo "$build_output" >&2
	exit 1
fi
if ! test_output="$(go test "$package" 2>&1)"; then
	echo "the unmutated suite does not pass, so nothing below would mean anything:" >&2
	echo "$test_output" >&2
	exit 1
fi

mutations=(
	"comments.go|||s/!strings\.HasPrefix\(text, \"#!\"\) && //|||shebang counted as a comment"
	"comments.go|||s/ && !directive\(text\)//|||preprocessor directives counted as comments"
	"comments.go|||s/return continuesBlock\(text\)/return true/|||dereferenced pointers counted as comments"
	"comments.go|||s/rest == \"\" \|\| //|||a bare asterisk no longer a comment"
	"comments.go|||s/strings\.HasPrefix\(rest, \"\/\"\) \|\|//|||a block comment close no longer a comment"
	"comments.go|||s/return directives\[word\]/return false/|||directive table never consulted"
	"comments.go|||s/strings\.HasPrefix\(text, \"\/\/\"\)/false/|||line comment marker dropped"
	"comments.go|||s/strings\.HasPrefix\(text, \"\/\*\"\)/false/|||block open marker dropped"
	"comments.go|||s/strings\.HasPrefix\(text, \"<!--\"\)/false/|||markup marker dropped"
	"comments.go|||s/\|\| skipped\(path\)//|||skipped extensions counted in diffs"
	"comments.go|||s/if skipped\(path\) \{\n\t\t\tcontinue\n\t\t\}//|||skipped extensions counted in untracked files"
	"comments.go|||s/\"\+\+\+ b\/\"/\"--- a\/\"/|||reads the pre-image filename"
	"comments.go|||s/path == \"\" \|\| //|||added lines counted before any filename"
	"comments.go|||s/strings\.TrimSpace\(added\)/added/|||indented comments missed"
	"comments.go|||s/strings\.TrimSpace\(line\)/line/|||indented comments missed in untracked files"
	"git.go|||s/os\.Lstat/os.Stat/|||symlinks followed instead of skipped"
	"git.go|||s/!info\.Mode\(\)\.IsRegular\(\) \|\| //|||anything that is not a file read as one"
	"git.go|||s/name == \"\" \|\| skipped\(name\)/name == \"\"/|||skipped extensions spend the read quota"
	"git.go|||s/len\(files\) >= maxUntrackedFiles/len(files) >= maxUntrackedFiles*2/|||read quota raised past its limit"
	"git.go|||s/maxFileBytes      = 1 << 20/maxFileBytes      = 1 << 40/|||file size limit raised past its limit"
	"state.go|||s/fs\.ErrNotExist/fs.ErrInvalid/|||a project nobody has seen reported as an error"
	"decide.go|||s/sinceBaseline > prev\.NudgedAtCount/sinceBaseline >= prev.NudgedAtCount/|||nudges when the count merely holds"
	"decide.go|||s/sinceBaseline > prev\.NudgedAtCount/sinceBaseline > 0/|||high-water mark ignored"
	"decide.go|||s/prev\.Baseline == \"\"/prev.Baseline != \"\"/|||first-run check inverted"
	"decide.go|||s/if nudge && prev\.Baseline == head \{/if nudge {/|||mark stored against the wrong revision after a commit"
	"decide.go|||s/return nudge, prev\.advance/return false, prev.advance/|||a nudge across a commit reported as quiet"
	"decide.go|||s/if prev\.Baseline != head \{\n\t\treturn state\{Baseline: head, NudgedAtCount: sinceHead\}\n\t\}//|||mark not restated when the baseline moves"
	"decide.go|||s/return state\{Baseline: head, NudgedAtCount: sinceHead\}/return state{Baseline: head}/|||mark zeroed instead of restated"
	"decide.go|||s/prev\.Baseline != head \{/prev.Baseline == head {/|||baseline-moved check inverted"
	"nudge.go|||s/len\(comments\) == 0/len(comments) != 0/|||empty-file-list check inverted"
	"nudge.go|||s/slices\.Sort\(files\)/slices.SortFunc(files, func(a, b string) int { return strings.Compare(b, a) })/|||file list sorted the wrong way"
	"turn.go|||s/!= \"done\" \&\& event\.Data\.AgentStatus != \"idle\"/!= \"done\" || event.Data.AgentStatus != \"idle\"/|||every status treated as a turn end"
	"turn.go|||s/ \&\& event\.Data\.AgentStatus != \"idle\"//|||idle no longer a turn end"
	"turn.go|||s/context\.FocusedPaneID != event\.Data\.PaneID/context.FocusedPaneID == event.Data.PaneID/|||pane match inverted"
	"turn.go|||s/if context\.FocusedPaneID != event\.Data\.PaneID \{\n\t\treturn turn\{\}, false\n\t\}//|||pane match guard removed"
	"turn.go|||s/ \|\| context\.FocusedPaneCwd == \"\"//|||missing working directory accepted"
)

gaps=0
for entry in "${mutations[@]}"; do
	file="cmd/herdr-kibitzr/${entry%%|||*}"
	rest="${entry#*|||}"
	expression="${rest%%|||*}"
	description="${rest#*|||}"

	# Every file, not just the one being mutated. Restoring only that one leaves
	# the previous iteration's mutation in place in another file, and the suite
	# then stays red for reasons that have nothing to do with this mutation.
	cp "$source_tree"/cmd/herdr-kibitzr/*.go cmd/herdr-kibitzr/
	perl -0777 -pi -e "$expression" "$file"

	if diff -q "$source_tree/$file" "$file" >/dev/null 2>&1; then
		echo "NOT APPLIED  $description"
		gaps=$((gaps + 1))
		continue
	fi

	# Without this the next line could not tell a test failure from a mutation
	# that produced code the compiler rejects, and would call both a pass.
	if ! go build "$package" >/dev/null 2>&1; then
		echo "NOT BUILT    $description"
		gaps=$((gaps + 1))
		continue
	fi

	if go test "$package" >/dev/null 2>&1; then
		echo "SURVIVED     $description"
		gaps=$((gaps + 1))
	else
		echo "caught       $description"
	fi
done

cp "$source_tree"/cmd/herdr-kibitzr/*.go cmd/herdr-kibitzr/
echo
echo "${#mutations[@]} mutations, $gaps uncaught"
[[ $gaps -eq 0 ]]
