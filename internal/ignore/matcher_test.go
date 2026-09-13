package ignore_test

import (
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/ignore"
)

func TestMatcherRespectsOrderingAndNegation(t *testing.T) {
	rules := []ignore.IgnoreRule{
		{BaseDir: ".", Pattern: "*.txt", Negated: false},
		{BaseDir: ".", Pattern: "keep.txt", Negated: true},
	}
	matcher := ignore.NewMatcher(rules)

	ignored, err := matcher.Ignored("keep.txt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignored {
		t.Fatal("expected keep.txt to be unignored")
	}

	ignored, err = matcher.Ignored("skip.txt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ignored {
		t.Fatal("expected skip.txt to be ignored")
	}
}

func TestMatcherDirectoryOnlyRuleMatchesParentDirectoryForFile(t *testing.T) {
	rules := []ignore.IgnoreRule{{BaseDir: ".", Pattern: "dist", DirectoryOnly: true}}
	matcher := ignore.NewMatcher(rules)

	ignored, err := matcher.Ignored("dist/main.js", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ignored {
		t.Fatal("expected file under dist to be ignored")
	}
}

func TestMatcherDirectoryOnlyRuleDoesNotMatchRootFile(t *testing.T) {
	rules := []ignore.IgnoreRule{{BaseDir: ".", Pattern: "dist", DirectoryOnly: true}}
	matcher := ignore.NewMatcher(rules)

	ignored, err := matcher.Ignored("main.js", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignored {
		t.Fatal("expected root file to remain included")
	}
}

func TestMatcherRespectsBaseDirScoping(t *testing.T) {
	rules := []ignore.IgnoreRule{{BaseDir: "src", Pattern: "*.go"}}
	matcher := ignore.NewMatcher(rules)

	ignored, err := matcher.Ignored("pkg/main.go", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignored {
		t.Fatal("expected file outside base dir to remain included")
	}

	ignored, err = matcher.Ignored("src/pkg/main.go", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ignored {
		t.Fatal("expected file under base dir to be ignored")
	}

	ignored, err = matcher.Ignored("src", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignored {
		t.Fatal("expected base directory path itself to remain included")
	}
}

func TestMatcherReturnsPatternErrorWithSourceAndLine(t *testing.T) {
	rules := []ignore.IgnoreRule{{BaseDir: ".", Source: ".ignore", Line: 7, Pattern: "["}}
	matcher := ignore.NewMatcher(rules)

	_, err := matcher.Ignored("file.txt", false)
	if err == nil {
		t.Fatal("expected invalid pattern error")
	}
	if !strings.Contains(err.Error(), ".ignore:7: invalid ignore pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}
