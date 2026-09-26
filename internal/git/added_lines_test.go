//nolint:testpackage
package git

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSelectAddedLinesModeOffIsNoOp(t *testing.T) {
	runCommandCalls := 0
	setSelectionRunCommandHook(t, func(string, ...string) (string, error) {
		runCommandCalls++

		return "", nil
	})

	addedLines, err := SelectAddedLines(CandidateSelectionRequest{Mode: "off", WorkingDir: "."})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if addedLines != nil {
		t.Fatalf("expected nil added-lines map, got %#v", addedLines)
	}
	if runCommandCalls != 0 {
		t.Fatalf("expected no git command calls, got %d", runCommandCalls)
	}
}

func TestSelectAddedLinesStagedUsesCachedDiffAndParsesHunks(t *testing.T) {
	var gotDir string
	var gotArgs []string
	setSelectionRunCommandHook(t, func(dir string, args ...string) (string, error) {
		gotDir = dir
		gotArgs = append([]string{}, args...)

		return "diff --git a/pkg/alpha.go b/pkg/alpha.go\n" +
			"index 1111111..2222222 100644\n" +
			"--- a/pkg/alpha.go\n" +
			"+++ b/pkg/alpha.go\n" +
			"@@ -1,0 +2,2 @@\n" +
			"+one\n" +
			"+two\n" +
			"@@ -10 +12 @@\n" +
			"-old\n" +
			"+new\n" +
			"diff --git a/pkg/beta.go b/pkg/beta.go\n" +
			"index 0000000..3333333 100644\n" +
			"--- /dev/null\n" +
			"+++ b/pkg/beta.go\n" +
			"@@ -0,0 +1 @@\n" +
			"+content\n", nil
	})

	addedLines, err := SelectAddedLines(CandidateSelectionRequest{Mode: "staged", WorkingDir: "/tmp/repo"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gotDir != "/tmp/repo" {
		t.Fatalf("expected command dir %q, got %q", "/tmp/repo", gotDir)
	}
	wantArgs := []string{"diff", "--cached", "--unified=0", "--no-color", "--no-prefix", "--diff-filter=ACMR"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, gotArgs)
	}

	assertLineSet(t, addedLines, "pkg/alpha.go", []int{2, 3, 12})
	assertLineSet(t, addedLines, "pkg/beta.go", []int{1})
}

func TestSelectAddedLinesDiffRequiresTarget(t *testing.T) {
	setSelectionRunCommandHook(t, func(string, ...string) (string, error) {
		t.Fatal("expected git command not to be invoked")

		return "", nil
	})

	_, err := SelectAddedLines(CandidateSelectionRequest{Mode: "diff", WorkingDir: "."})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "git mode diff requires diff target" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectAddedLinesReturnsModeSpecificCommandErrors(t *testing.T) {
	setSelectionRunCommandHook(t, func(string, ...string) (string, error) {
		return "", errors.New("fatal")
	})

	_, err := SelectAddedLines(CandidateSelectionRequest{Mode: "staged", WorkingDir: "."})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "git mode staged failed to resolve added lines" {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = SelectAddedLines(CandidateSelectionRequest{
		Mode:       "diff",
		DiffTarget: "HEAD~1..HEAD",
		WorkingDir: ".",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "git mode diff failed to resolve added lines for target \"HEAD~1..HEAD\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectAddedLinesRejectsPathOutsideRepositoryRoot(t *testing.T) {
	setSelectionRunCommandHook(t, func(string, ...string) (string, error) {
		return "+++ b/../outside.go\n@@ -0,0 +1 @@\n+bad\n", nil
	})

	_, err := SelectAddedLines(CandidateSelectionRequest{Mode: "staged", WorkingDir: "."})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "git mode staged failed to resolve added lines: ") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAddedLinesHandlesDiffLinesOver64KiB(t *testing.T) {
	oversizedLine := "+" + strings.Repeat("A", 128*1024)
	diff := strings.Join([]string{
		"+++ pkg/big.json",
		"@@ -0,0 +1 @@",
		oversizedLine,
		"+++ pkg/alpha.go",
		"@@ -5,0 +6,2 @@",
		"+alpha",
		"+beta",
	}, "\n")

	addedLines, err := parseAddedLines(diff)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertLineSet(t, addedLines, "pkg/big.json", []int{1})
	assertLineSet(t, addedLines, "pkg/alpha.go", []int{6, 7})
}

func TestParseAddedLinesReturnsEmptyMapForFilesWithoutAddedHunks(t *testing.T) {
	addedLines, err := parseAddedLines(
		"diff --git a/pkg/alpha.go b/pkg/alpha.go\n" +
			"index 1111111..2222222 100644\n" +
			"--- a/pkg/alpha.go\n" +
			"+++ b/pkg/alpha.go\n" +
			"@@ -10,2 +10,0 @@\n" +
			"-old\n" +
			"-value\n",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(addedLines) != 0 {
		t.Fatalf("expected no added lines, got %#v", addedLines)
	}
}

func assertLineSet(t *testing.T, addedLines map[string]map[int]struct{}, filePath string, want []int) {
	t.Helper()

	set, ok := addedLines[filePath]
	if !ok {
		t.Fatalf("expected added lines for %q, got %#v", filePath, addedLines)
	}
	if len(set) != len(want) {
		t.Fatalf("expected %d lines for %q, got %d", len(want), filePath, len(set))
	}
	for _, line := range want {
		if _, ok := set[line]; !ok {
			t.Fatalf("expected line %d for %q, got %#v", line, filePath, set)
		}
	}
}
