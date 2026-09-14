package ignore_test

import (
	"testing"

	"github.com/reglint/reglint/internal/ignore"
)

func TestParseIgnoresCommentsAndHandlesEscapes(t *testing.T) {
	content := "# comment\n\n\\#literal\n\\!literal\n!negated\n/anchored\nassets/\n"

	rules, err := ignore.Parse("base", "root/.ignore", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 5 {
		t.Fatalf("expected 5 rules, got %d", len(rules))
	}

	expected := []struct {
		line          int
		pattern       string
		negated       bool
		directoryOnly bool
	}{
		{line: 3, pattern: "#literal"},
		{line: 4, pattern: "!literal"},
		{line: 5, pattern: "negated", negated: true},
		{line: 6, pattern: "/anchored"},
		{line: 7, pattern: "assets", directoryOnly: true},
	}

	for i, want := range expected {
		rule := rules[i]
		if rule.BaseDir != "base" {
			t.Fatalf("expected base dir %q, got %q", "base", rule.BaseDir)
		}
		if rule.Source != "root/.ignore" {
			t.Fatalf("expected source %q, got %q", "root/.ignore", rule.Source)
		}
		if rule.Line != want.line {
			t.Fatalf("expected line %d, got %d", want.line, rule.Line)
		}
		if rule.Pattern != want.pattern {
			t.Fatalf("expected pattern %q, got %q", want.pattern, rule.Pattern)
		}
		if rule.Negated != want.negated {
			t.Fatalf("expected negated=%v, got %v", want.negated, rule.Negated)
		}
		if rule.DirectoryOnly != want.directoryOnly {
			t.Fatalf("expected directoryOnly=%v, got %v", want.directoryOnly, rule.DirectoryOnly)
		}
	}
}

func TestParseNormalizesLineEndings(t *testing.T) {
	content := "first\r\nsecond\r\n"

	rules, err := ignore.Parse("", "root/.ignore", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Line != 1 || rules[1].Line != 2 {
		t.Fatalf("unexpected line numbers: %d, %d", rules[0].Line, rules[1].Line)
	}
}

func TestParseRejectsInvalidPatternWithSourceLine(t *testing.T) {
	_, err := ignore.Parse("", "root/.ignore", "[\n")
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
	if err.Error() != "root/.ignore:1: invalid ignore pattern" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSkipsEmptyNegationAndRootOnly(t *testing.T) {
	content := "!\n/\n"

	rules, err := ignore.Parse("", "root/.ignore", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}
