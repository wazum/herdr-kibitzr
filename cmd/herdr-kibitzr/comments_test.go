package main

import "testing"

func TestAddedCommentsFindsALineComment(t *testing.T) {
	got := addedComments([]addition{
		{path: "src/total.go", text: "func total() int {\n\t// running sum\n\tsum := 0\n}"},
	})

	assertComments(t, got, map[string][]string{"src/total.go": {"// running sum"}})
}

func TestAddedCommentsFindsEveryMarker(t *testing.T) {
	got := addedComments([]addition{{path: "a.php", text: "" +
		"# hash\n" +
		"/** docblock\n" +
		" * continued\n" +
		" */\n" +
		"/// triple slash\n" +
		"code();\n" +
		"<!-- markup -->\n"}})

	assertComments(t, got, map[string][]string{"a.php": {
		"# hash",
		"/** docblock",
		"* continued",
		"*/",
		"/// triple slash",
		"<!-- markup -->",
	}})
}

func TestAddedCommentsIgnoresShebang(t *testing.T) {
	got := addedComments([]addition{{path: "run.sh", text: "#!/usr/bin/env bash\n"}})

	assertComments(t, got, map[string][]string{})
}

func TestAddedCommentsSkipsProseAndDataFiles(t *testing.T) {
	got := addedComments([]addition{
		{path: "README.md", text: "# Heading\n"},
		{path: "config.yaml", text: "# a key\n"},
		{path: "main.go", text: "// kept\n"},
	})

	assertComments(t, got, map[string][]string{"main.go": {"// kept"}})
}

func TestAddedCommentsFindsIndentedComments(t *testing.T) {
	got := addedComments([]addition{
		{path: "src/Order.php", text: "class Order\n{\n    /** @var int */\n    private int $total;\n}"},
	})

	assertComments(t, got, map[string][]string{"src/Order.php": {"/** @var int */"}})
}

func TestAddedCommentsIgnoresCodeThatStartsWithAnAsterisk(t *testing.T) {
	got := addedComments([]addition{{path: "main.c", text: "" +
		"*ptr = value;\n" +
		"**handle = other;\n" +
		"* still a block comment\n" +
		"*/\n" +
		"*\n"}})

	assertComments(t, got, map[string][]string{"main.c": {
		"* still a block comment",
		"*/",
		"*",
	}})
}

func TestAddedCommentsIgnoresPreprocessorDirectives(t *testing.T) {
	got := addedComments([]addition{{path: "main.c", text: "" +
		"#include <stdio.h>\n" +
		"#define WIDTH 80\n" +
		"#ifndef GUARD\n" +
		"#endif\n" +
		"#pragma once\n" +
		"# a real comment\n" +
		"#also a real comment\n"}})

	assertComments(t, got, map[string][]string{"main.c": {
		"# a real comment",
		"#also a real comment",
	}})
}

func TestAddedCommentsGathersEveryWriteToOneFile(t *testing.T) {
	got := addedComments([]addition{
		{path: "main.go", text: "// first edit\n"},
		{path: "main.go", text: "// second edit\n"},
	})

	assertComments(t, got, map[string][]string{"main.go": {"// first edit", "// second edit"}})
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
