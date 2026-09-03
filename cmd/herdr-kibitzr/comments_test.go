package main

import "testing"

func TestAddedCommentsFindsLineComment(t *testing.T) {
	diff := `diff --git a/src/total.go b/src/total.go
--- a/src/total.go
+++ b/src/total.go
@@ -1,3 +1,4 @@
 func total() int {
+	// running sum
 	sum := 0
 }
`

	got := addedComments(diff, nil)

	want := map[string][]string{"src/total.go": {"// running sum"}}
	assertComments(t, got, want)
}

func TestAddedCommentsFindsEveryMarker(t *testing.T) {
	diff := `+++ b/a.php
+# hash
+/** docblock
+ * continued
+ */
+/// triple slash
++++ not a marker
+<!-- markup -->
`

	got := addedComments(diff, nil)

	want := map[string][]string{"a.php": {
		"# hash",
		"/** docblock",
		"* continued",
		"*/",
		"/// triple slash",
		"<!-- markup -->",
	}}
	assertComments(t, got, want)
}

func TestAddedCommentsIgnoresShebang(t *testing.T) {
	diff := "+++ b/run.sh\n+#!/usr/bin/env bash\n"

	got := addedComments(diff, nil)

	assertComments(t, got, map[string][]string{})
}

func TestAddedCommentsSkipsProseAndDataFiles(t *testing.T) {
	diff := "+++ b/README.md\n+# Heading\n+++ b/config.yaml\n+# a key\n+++ b/main.go\n+// kept\n"

	got := addedComments(diff, nil)

	assertComments(t, got, map[string][]string{"main.go": {"// kept"}})
}

func TestAddedCommentsCountsEveryLineOfAnUntrackedFile(t *testing.T) {
	untracked := map[string]string{
		"src/New.php": "<?php\n// fresh\nclass New {}\n",
		"notes.md":    "# heading\n",
	}

	got := addedComments("", untracked)

	assertComments(t, got, map[string][]string{"src/New.php": {"// fresh"}})
}

func TestAddedCommentsIgnoresLinesBeforeAnyFilename(t *testing.T) {
	diff := "+ // stray line with no file header\n+++ b/main.go\n+// kept\n"

	got := addedComments(diff, nil)

	assertComments(t, got, map[string][]string{"main.go": {"// kept"}})
}

func TestAddedCommentsFindsIndentedCommentsInUntrackedFiles(t *testing.T) {
	untracked := map[string]string{
		"src/Order.php": "<?php\nclass Order\n{\n    /** @var int */\n    private int $total;\n}\n",
	}

	got := addedComments("", untracked)

	assertComments(t, got, map[string][]string{"src/Order.php": {"/** @var int */"}})
}

func TestAddedCommentsIgnoresCodeThatStartsWithAnAsterisk(t *testing.T) {
	diff := "+++ b/main.c\n" +
		"+*ptr = value;\n" +
		"+**handle = other;\n" +
		"+* still a block comment\n" +
		"+*/\n" +
		"+*\n"

	got := addedComments(diff, nil)

	assertComments(t, got, map[string][]string{"main.c": {
		"* still a block comment",
		"*/",
		"*",
	}})
}

func TestAddedCommentsIgnoresPreprocessorDirectives(t *testing.T) {
	diff := "+++ b/main.c\n" +
		"+#include <stdio.h>\n" +
		"+#define WIDTH 80\n" +
		"+#ifndef GUARD\n" +
		"+#endif\n" +
		"+#pragma once\n" +
		"+# a real comment\n" +
		"+#also a real comment\n"

	got := addedComments(diff, nil)

	assertComments(t, got, map[string][]string{"main.c": {
		"# a real comment",
		"#also a real comment",
	}})
}

func assertComments(t *testing.T, got, want map[string][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d files %v, want %d files %v", len(got), got, len(want), want)
	}
	for path, wantLines := range want {
		gotLines, ok := got[path]
		if !ok {
			t.Fatalf("no comments for %q, got %v", path, got)
		}
		if len(gotLines) != len(wantLines) {
			t.Fatalf("%q: got %v, want %v", path, gotLines, wantLines)
		}
		for i := range wantLines {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("%q line %d: got %q, want %q", path, i, gotLines[i], wantLines[i])
			}
		}
	}
}
