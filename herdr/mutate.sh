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
	"comments.go|||s/if skipped\(one\.path\) \{\n\t\t\tcontinue\n\t\t\}//|||skipped extensions counted"
	"comments.go|||s/carried := commentTally\(one\.replaced\)/carried := commentTally(\"\")/|||comments an edit carried along counted as new"
	"comments.go|||s/if carried\[text\] > 0 \{/if carried[text] < 0 {/|||the carried tally never matches"
	"comments.go|||s/carried\[text\]--/carried[text] -= 0/|||a repeated comment hidden by its first copy"
	"nudge.go|||s/if commit != \"\" \{/if commit == \"\" {/|||the amend hint offered backwards"
	"decide.go|||s/prev\.LastHead == \"\" \|\| prev\.LastHead == head/prev.LastHead == \"\"/|||a commit reported when none happened"
	"decide.go|||s/if prev\.LastHead == \"\" \|\| prev\.LastHead == head \{\n\t\treturn \"\"\n\t\}//|||every turn reported as a commit"
	"comments.go|||s/strings\.TrimSpace\(line\)/line/|||indented comments missed"
	"recent.go|||s/\"\+\+\+ b\/\"/\"--- a\/\"/|||reads the pre-image filename"
	"recent.go|||s/line == \"\+\+\+ \/dev\/null\"/false/|||a deletion header read as content"
	"recent.go|||s/!ok \|\| path == \"\"/!ok/|||added lines counted before any filename"
	"recent.go|||s/after := time\.Unix\(0, since\)/after := time.Unix(0, since*0)/|||every changed file blamed on this agent"
	"recent.go|||s/info\.ModTime\(\)\.After\(after\)/info.ModTime().Before(after)/|||the time window runs backwards"
	"recent.go|||s/return nil, now, nil/return nil, cursor, nil/|||a first look never advances its cursor"
	"claude.go|||s/case \"Write\":/case \"WriteNothing\":/|||a whole new file goes unread"
	"claude.go|||s/case \"Edit\":/case \"EditNothing\":/|||an edit goes unread"
	"claude.go|||s/part\.Type != \"tool_use\"/part.Type == \"tool_use\"/|||tool calls skipped and prose read instead"
	"claude.go|||s/if err != nil \{\n\t\treturn nil, strconv\.FormatInt\(end, 10\), nil\n\t\}//|||an unread session reads its whole history"
	"git.go|||s/os\.Lstat/os.Stat/|||symlinks followed instead of skipped"
	"git.go|||s/!info\.Mode\(\)\.IsRegular\(\) \|\| //|||anything that is not a file read as one"
	"git.go|||s/name == \"\" \|\| skipped\(name\)/name == \"\"/|||skipped extensions spend the read quota"
	"git.go|||s/len\(files\) >= maxUntrackedFiles/len(files) >= maxUntrackedFiles*2/|||read quota raised past its limit"
	"git.go|||s/maxFileBytes      = 1 << 20/maxFileBytes      = 1 << 40/|||file size limit raised past its limit"
	"state.go|||s/fs\.ErrNotExist/fs.ErrInvalid/|||a project nobody has seen reported as an error"
	"composer.go|||s/case \"\\\\x1b\\[2m\":/case \"\\\\x1b[999m\":/|||a dim suggestion read as typed text"
	"composer.go|||s/if !dim \{\n\t\t\tkept\.WriteString\(line\[:match\[0\]\]\)\n\t\t\}//|||typing over a suggestion read as empty"
	"composer.go|||s/for i := len\(lines\) - 1; i >= 0; i--/for i := 0; i < len(lines); i++/|||reads the first prompt line, not the composer"
	"decide.go|||s/return prev\.LastStatus != status/return true/|||a title change treated as a turn end"
	"decide.go|||s/return prev\.LastStatus != status/return false/|||no turn ever ends"
	"decide.go|||s/if count > 0 \{/if count >= 0 {/|||nudges about no comments at all"
	"comments.go|||s/strings\.HasPrefix\(text, \`\"\"\"\`\)/false/|||python docstrings missed"
	"comments.go|||s/strings\.HasPrefix\(text, \"'''\"\)/false/|||single quoted docstrings missed"
	"decide.go|||s/return true, state\{Cursor: cursor, AwaitingCleanup: true\}/return false, state{Cursor: cursor, AwaitingCleanup: true}/|||a nudge reported as quiet"
	"decide.go|||s/if prev\.AwaitingCleanup \{/if false {/|||no uncontested cleanup turn"
	"decide.go|||s/AwaitingCleanup: true/AwaitingCleanup: false/|||a nudge grants no cleanup turn"
	"decide.go|||s/AwaitingCleanup: count == 0/AwaitingCleanup: false/|||any event at all spends the cleanup pass"
	"decide.go|||s/AwaitingCleanup: count == 0/AwaitingCleanup: true/|||the cleanup pass is never spent"
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
