//nolint:testpackage
package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/scan"
)

func TestGoldenGitHubOutput(t *testing.T) {
	t.Parallel()

	if (GitHubFormatter{}).Name() != "github" {
		t.Fatalf("formatter name must be github")
	}

	var buffer bytes.Buffer
	if err := WriteGitHub(goldenSarifResult(), goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNoANSIControlSequences(t, buffer.Bytes())
	assertGoldenBytes(t, "github.txt", buffer.Bytes())
}

func TestGitHubEscaping(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{
				Message:   "100% done\nsecond\rline",
				Severity:  "error",
				FilePath:  "dir:name,file.go",
				Line:      1,
				Column:    1,
				MatchText: "SENSITIVE",
			},
		},
	}

	var buffer bytes.Buffer
	if err := WriteGitHub(result, goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "::error file=dir%3Aname%2Cfile.go,line=1,col=1,title=RC0001::100%25 done%0Asecond%0Dline\n"
	if buffer.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buffer.String())
	}
	assertNoANSIControlSequences(t, buffer.Bytes())
	if strings.Contains(buffer.String(), "SENSITIVE") {
		t.Fatalf("output must not contain raw match text")
	}
}

func TestGitHubSeverityMapping(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"error":   "::error ",
		"warning": "::warning ",
		"notice":  "::notice ",
		"info":    "::notice ",
	}

	matches := make([]scan.Match, 0, len(cases))
	for severity := range cases {
		matches = append(matches, scan.Match{
			Message:   "msg " + severity,
			Severity:  severity,
			FilePath:  "f.go",
			Line:      1,
			Column:    1,
			MatchText: "match",
		})
	}

	var buffer bytes.Buffer
	if err := WriteGitHub(scan.Result{Matches: matches}, goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for severity, prefix := range cases {
		if !strings.Contains(buffer.String(), prefix+"file=f.go,line=1,col=1,title=RC0001::msg "+severity+"\n") {
			t.Fatalf("severity %q not rendered with %q prefix: %q", severity, prefix, buffer.String())
		}
	}
}

func TestGitHubCapsDropBeyondTenPerLevel(t *testing.T) {
	t.Parallel()

	matches := make([]scan.Match, 0, 11*3)
	for level := range 11 {
		for _, severity := range []string{"error", "warning", "notice"} {
			matches = append(matches, scan.Match{
				Message:  "msg",
				Severity: severity,
				FilePath: "f.go",
				Line:     level + 1,
				Column:   1,
			})
		}
	}

	var buffer bytes.Buffer
	if err := WriteGitHub(scan.Result{Matches: matches}, goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n")
	if len(lines) != 31 {
		t.Fatalf("expected 31 lines (10 per level + summary), got %d", len(lines))
	}
	for _, level := range []string{"error", "warning", "notice"} {
		if got := strings.Count(buffer.String(), "::"+level+" file="); got != 10 {
			t.Fatalf("expected 10 %s annotations, got %d", level, got)
		}
	}
	expectedSummary := "::notice title=RegLint::30 of 33 matches shown; " +
		"GitHub limits annotations to 10 per severity per step. " +
		"Use --format json or sarif for the full list.\n"
	if lines[30]+"\n" != expectedSummary {
		t.Fatalf("expected summary %q, got %q", expectedSummary, lines[30]+"\n")
	}
}

func TestGitHubNoSummaryAtExactCap(t *testing.T) {
	t.Parallel()

	matches := make([]scan.Match, 0, 10*3)
	for level := range 10 {
		for _, severity := range []string{"error", "warning", "notice"} {
			matches = append(matches, scan.Match{
				Message:  "msg",
				Severity: severity,
				FilePath: "f.go",
				Line:     level + 1,
				Column:   1,
			})
		}
	}

	var buffer bytes.Buffer
	if err := WriteGitHub(scan.Result{Matches: matches}, goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(buffer.String(), "title=RegLint") {
		t.Fatalf("summary must not be emitted when nothing was dropped: %q", buffer.String())
	}
	if got := strings.Count(buffer.String(), "\n"); got != 30 {
		t.Fatalf("expected 30 lines, got %d", got)
	}
}

func TestGitHubZeroMatches(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	if err := WriteGitHub(scan.Result{}, goldenSarifRules(), &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buffer.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buffer.String())
	}
}

type githubFailingWriter struct{}

func (githubFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestGitHubWriterErrorPropagates(t *testing.T) {
	t.Parallel()

	err := WriteGitHub(goldenSarifResult(), goldenSarifRules(), githubFailingWriter{})
	if err == nil || err.Error() != "write failed" {
		t.Fatalf("expected writer error to propagate, got %v", err)
	}
}
