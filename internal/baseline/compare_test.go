//nolint:testpackage // Validate internal behavior directly.
package baseline

import (
	"reflect"
	"testing"

	"github.com/reglint/reglint/internal/scan"
)

func TestCompareSuppressesEqualCounts(t *testing.T) {
	t.Parallel()

	document := Document{
		SchemaVersion: 1,
		Entries: []Entry{
			{FilePath: "src/a.go", Message: "m1", Count: 2},
			{FilePath: "src/b.go", Message: "m2", Count: 1},
		},
	}

	current := []scan.Match{
		{FilePath: "src/a.go", Message: "m1", Severity: "warning", Line: 2, Column: 1},
		{FilePath: "src/b.go", Message: "m2", Severity: "error", Line: 1, Column: 1},
		{FilePath: "src/a.go", Message: "m1", Severity: "warning", Line: 8, Column: 3},
	}

	result := Compare(current, document)

	if len(result.Regressions) != 0 {
		t.Fatalf("expected no regressions, got %d", len(result.Regressions))
	}
	if result.SuppressedCount != 3 {
		t.Fatalf("expected suppressed count 3, got %d", result.SuppressedCount)
	}
	if result.ImprovementsCount != 0 {
		t.Fatalf("expected improvements count 0, got %d", result.ImprovementsCount)
	}
}

func TestCompareReturnsOnlyExcessMatchesInDeterministicOrder(t *testing.T) {
	t.Parallel()

	document := Document{
		SchemaVersion: 1,
		Entries:       []Entry{{FilePath: "src/a.go", Message: "m1", Count: 1}},
	}

	current := []scan.Match{
		{FilePath: "src/a.go", Message: "m1", Severity: "warning", Line: 20, Column: 1},
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 5, Column: 1},
		{FilePath: "src/a.go", Message: "m1", Severity: "notice", Line: 10, Column: 1},
	}

	result := Compare(current, document)

	if result.SuppressedCount != 1 {
		t.Fatalf("expected suppressed count 1, got %d", result.SuppressedCount)
	}
	if result.ImprovementsCount != 0 {
		t.Fatalf("expected improvements count 0, got %d", result.ImprovementsCount)
	}
	if len(result.Regressions) != 2 {
		t.Fatalf("expected 2 regressions, got %d", len(result.Regressions))
	}
	if result.Regressions[0].Line != 10 {
		t.Fatalf("expected first regression line 10, got %d", result.Regressions[0].Line)
	}
	if result.Regressions[1].Line != 20 {
		t.Fatalf("expected second regression line 20, got %d", result.Regressions[1].Line)
	}
}

func TestCompareCountsImprovementsForMissingBaselineMatches(t *testing.T) {
	t.Parallel()

	document := Document{
		SchemaVersion: 1,
		Entries: []Entry{
			{FilePath: "src/a.go", Message: "m1", Count: 3},
			{FilePath: "src/b.go", Message: "m2", Count: 2},
		},
	}

	current := []scan.Match{
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 1, Column: 1},
	}

	result := Compare(current, document)

	if len(result.Regressions) != 0 {
		t.Fatalf("expected no regressions, got %d", len(result.Regressions))
	}
	if result.SuppressedCount != 1 {
		t.Fatalf("expected suppressed count 1, got %d", result.SuppressedCount)
	}
	if result.ImprovementsCount != 4 {
		t.Fatalf("expected improvements count 4, got %d", result.ImprovementsCount)
	}
}

func TestCompareDeterministicForEquivalentInputs(t *testing.T) {
	t.Parallel()

	documentA := Document{
		SchemaVersion: 1,
		Entries: []Entry{
			{FilePath: "src/b.go", Message: "m2", Count: 1},
			{FilePath: "src/a.go", Message: "m1", Count: 1},
		},
	}
	documentB := Document{
		SchemaVersion: 1,
		Entries: []Entry{
			{FilePath: "src/a.go", Message: "m1", Count: 1},
			{FilePath: "src/b.go", Message: "m2", Count: 1},
		},
	}

	currentA := []scan.Match{
		{FilePath: "src/b.go", Message: "m2", Severity: "warning", Line: 8, Column: 2},
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 3, Column: 1},
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 2, Column: 1},
	}
	currentB := []scan.Match{
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 2, Column: 1},
		{FilePath: "src/b.go", Message: "m2", Severity: "warning", Line: 8, Column: 2},
		{FilePath: "src/a.go", Message: "m1", Severity: "error", Line: 3, Column: 1},
	}

	resultA := Compare(currentA, documentA)
	resultB := Compare(currentB, documentB)

	if !reflect.DeepEqual(resultA, resultB) {
		t.Fatalf("expected deterministic results, got %+v and %+v", resultA, resultB)
	}
}

func TestSeverityRankOrdersKnownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		severity string
		want     int
	}{
		{name: "error", severity: "error", want: severityRankError},
		{name: "warning", severity: "warning", want: severityRankWarning},
		{name: "notice", severity: "notice", want: severityRankNotice},
		{name: "info", severity: "info", want: severityRankInfo},
		{name: "unknown", severity: "critical", want: severityRankUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := severityRank(tt.severity)
			if got != tt.want {
				t.Fatalf("expected rank %d, got %d", tt.want, got)
			}
		})
	}
}

func TestCompareMatchUsesTieBreakersInOrder(t *testing.T) {
	t.Parallel()

	base := scan.Match{
		FilePath:  "src/a.go",
		Line:      10,
		Column:    4,
		Severity:  "warning",
		Message:   "m",
		Root:      "/repo",
		MatchText: "token",
		RuleIndex: 2,
	}

	tests := []struct {
		name  string
		left  scan.Match
		right scan.Match
	}{
		{name: "file path", left: withFilePath(base, "src/a.go"), right: withFilePath(base, "src/b.go")},
		{name: "line", left: withLine(base, 5), right: withLine(base, 6)},
		{name: "column", left: withColumn(base, 1), right: withColumn(base, 2)},
		{name: "severity", left: withSeverity(base, "error"), right: withSeverity(base, "warning")},
		{name: "message", left: withMessage(base, "a"), right: withMessage(base, "b")},
		{name: "root", left: withRoot(base, "/a"), right: withRoot(base, "/b")},
		{name: "match text", left: withMatchText(base, "aaa"), right: withMatchText(base, "bbb")},
		{name: "rule index", left: withRuleIndex(base, 1), right: withRuleIndex(base, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if compareMatch(tt.left, tt.right) >= 0 {
				t.Fatalf("expected left to sort before right for %s", tt.name)
			}
			if compareMatch(tt.right, tt.left) <= 0 {
				t.Fatalf("expected right to sort after left for %s", tt.name)
			}
		})
	}
}

func TestSortCurrentMatchesReturnsSortedCopy(t *testing.T) {
	t.Parallel()

	current := []scan.Match{
		{FilePath: "src/b.go", Message: "m2", Severity: "warning", Line: 8, Column: 2},
		{FilePath: "src/a.go", Message: "m1", Severity: "warning", Line: 3, Column: 1},
	}

	sorted := sortCurrentMatches(current)

	if len(sorted) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(sorted))
	}
	if sorted[0].FilePath != "src/a.go" {
		t.Fatalf("expected src/a.go first, got %q", sorted[0].FilePath)
	}
	if current[0].FilePath != "src/b.go" {
		t.Fatalf("expected original slice to remain unchanged, got %q", current[0].FilePath)
	}
}

func withFilePath(match scan.Match, filePath string) scan.Match {
	match.FilePath = filePath

	return match
}

func withLine(match scan.Match, line int) scan.Match {
	match.Line = line

	return match
}

func withColumn(match scan.Match, column int) scan.Match {
	match.Column = column

	return match
}

func withSeverity(match scan.Match, severity string) scan.Match {
	match.Severity = severity

	return match
}

func withMessage(match scan.Match, message string) scan.Match {
	match.Message = message

	return match
}

func withRoot(match scan.Match, root string) scan.Match {
	match.Root = root

	return match
}

func withMatchText(match scan.Match, matchText string) scan.Match {
	match.MatchText = matchText

	return match
}

func withRuleIndex(match scan.Match, ruleIndex int) scan.Match {
	match.RuleIndex = ruleIndex

	return match
}
