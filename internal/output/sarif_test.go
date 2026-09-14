//nolint:testpackage
package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool       sarifTool     `json:"tool"`
	ColumnKind string        `json:"columnKind"`
	Results    []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

func TestWriteSARIFNoMatches(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: nil,
		Stats: scan.Stats{
			FilesScanned: 0,
			FilesSkipped: 0,
			Matches:      0,
			DurationMs:   0,
		},
	}
	ruleSet := []rules.Rule{
		{
			Message:  "Avoid hardcoded token: $1",
			Regex:    "token",
			Severity: "error",
		},
	}

	var buffer bytes.Buffer
	if err := WriteSARIF(result, ruleSet, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := decodeSARIF(t, buffer.Bytes())
	if got.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Fatalf("unexpected schema: %s", got.Schema)
	}
	assertSARIFMetadata(t, got)
	assertSARIFRule(t, got, 0, "RC0001", "Avoid hardcoded token: $1")
	assertSARIFResultsCount(t, got, 0)
}

func TestWriteSARIFOrdersAndMapsResults(t *testing.T) {
	t.Parallel()

	ruleSet := sarifSampleRules()
	result := sarifSampleResult()

	var buffer bytes.Buffer
	if err := WriteSARIF(result, ruleSet, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := decodeSARIF(t, buffer.Bytes())
	assertSARIFMetadata(t, got)
	assertSARIFResultsCount(t, got, 3)

	first := got.Runs[0].Results[0]
	assertSARIFResult(t, first, "RC0001", "error")
	assertSARIFLocation(t, first, "a/file.go", 3, 4, 3, 6)
	assertSARIFResultLevel(t, got.Runs[0].Results[2], "warning")
}

func TestWriteSARIFDoesNotEmitANSIControlSequences(t *testing.T) {
	t.Parallel()

	ruleSet := sarifSampleRules()
	result := sarifSampleResult()

	var buffer bytes.Buffer
	if err := WriteSARIF(result, ruleSet, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNoANSIControlSequences(t, buffer.Bytes())
}

func sarifSampleRules() []rules.Rule {
	return []rules.Rule{
		{
			Message:  "Avoid hardcoded token: $1",
			Regex:    "token",
			Severity: "error",
		},
		{
			Message:  "Suspicious marker",
			Regex:    "marker",
			Severity: "warning",
		},
	}
}

func sarifSampleResult() scan.Result {
	return scan.Result{
		Matches: []scan.Match{
			{
				Message:   "Suspicious marker",
				Severity:  "warning",
				FilePath:  "b/file.go",
				Line:      2,
				Column:    1,
				MatchText: "marker",
				RuleIndex: 1,
			},
			{
				Message:   "Avoid hardcoded token: ab",
				Severity:  "error",
				FilePath:  "a/file.go",
				Line:      3,
				Column:    4,
				MatchText: "ab",
				RuleIndex: 0,
			},
			{
				Message:   "Suspicious marker",
				Severity:  "info",
				FilePath:  "a/file.go",
				Line:      3,
				Column:    4,
				MatchText: "info",
				RuleIndex: 1,
			},
		},
		Stats: scan.Stats{
			FilesScanned: 2,
			FilesSkipped: 0,
			Matches:      3,
			DurationMs:   12,
		},
	}
}

func decodeSARIF(t *testing.T, data []byte) sarifLog {
	t.Helper()

	var log sarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("failed to parse sarif output: %v", err)
	}

	return log
}

func assertSARIFMetadata(t *testing.T, log sarifLog) {
	t.Helper()

	if log.Version != "2.1.0" {
		t.Fatalf("unexpected version: %s", log.Version)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(log.Runs))
	}
	run := log.Runs[0]
	if run.ColumnKind != "unicodeCodePoints" {
		t.Fatalf("unexpected columnKind: %s", run.ColumnKind)
	}
	if run.Tool.Driver.Name != "RegLint" {
		t.Fatalf("unexpected driver name: %s", run.Tool.Driver.Name)
	}
}

func assertSARIFRule(t *testing.T, log sarifLog, index int, id, description string) {
	t.Helper()

	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) <= index {
		t.Fatalf("expected rule index %d, got %d", index, len(rules))
	}
	if rules[index].ID != id {
		t.Fatalf("unexpected rule id: %s", rules[index].ID)
	}
	if rules[index].ShortDescription.Text != description {
		t.Fatalf("unexpected rule description: %s", rules[index].ShortDescription.Text)
	}
}

func assertSARIFResultsCount(t *testing.T, log sarifLog, expected int) {
	t.Helper()

	if len(log.Runs[0].Results) != expected {
		t.Fatalf("expected %d results, got %d", expected, len(log.Runs[0].Results))
	}
}

func assertSARIFResult(t *testing.T, result sarifResult, ruleID, level string) {
	t.Helper()

	if result.RuleID != ruleID {
		t.Fatalf("unexpected rule id: %s", result.RuleID)
	}
	if result.Level != level {
		t.Fatalf("unexpected level: %s", result.Level)
	}
}

func assertSARIFResultLevel(t *testing.T, result sarifResult, level string) {
	t.Helper()

	if result.Level != level {
		t.Fatalf("unexpected level: %s", result.Level)
	}
}

func assertSARIFLocation(t *testing.T, result sarifResult, uri string, startLine, startColumn, endLine, endColumn int) {
	t.Helper()

	location := result.Locations[0].PhysicalLocation
	if location.ArtifactLocation.URI != uri {
		t.Fatalf("unexpected artifact uri: %s", location.ArtifactLocation.URI)
	}
	if location.Region.StartLine != startLine || location.Region.StartColumn != startColumn {
		t.Fatalf("unexpected region start: %d:%d", location.Region.StartLine, location.Region.StartColumn)
	}
	if location.Region.EndLine != endLine || location.Region.EndColumn != endColumn {
		t.Fatalf("unexpected region end: %d:%d", location.Region.EndLine, location.Region.EndColumn)
	}
}
