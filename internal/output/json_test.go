//nolint:testpackage
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

type jsonOutputResult struct {
	SchemaVersion int               `json:"schemaVersion"`
	Matches       []jsonOutputMatch `json:"matches"`
	Stats         jsonOutputStats   `json:"stats"`
}

type jsonOutputMatch struct {
	Message      string `json:"message"`
	Severity     string `json:"severity"`
	FilePath     string `json:"filePath"`
	AbsolutePath string `json:"absolutePath"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	MatchText    string `json:"matchText"`
}

type jsonOutputStats struct {
	FilesScanned int   `json:"filesScanned"`
	FilesSkipped int   `json:"filesSkipped"`
	Matches      int   `json:"matches"`
	DurationMs   int64 `json:"durationMs"`
}

func TestWriteJSONNoMatches(t *testing.T) {
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

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got jsonOutputResult
	if err := json.Unmarshal(buffer.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}

	if got.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: %d", got.SchemaVersion)
	}
	if len(got.Matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(got.Matches))
	}
	if got.Stats.FilesScanned != 0 || got.Stats.FilesSkipped != 0 || got.Stats.Matches != 0 || got.Stats.DurationMs != 0 {
		t.Fatalf("unexpected stats: %+v", got.Stats)
	}
}

func TestWriteJSONNoMatchesUsesEmptyArray(t *testing.T) {
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

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := decodeRawJSON(t, buffer.Bytes())
	matches, ok := raw["matches"].([]any)
	if !ok {
		t.Fatalf("expected matches array, got %T", raw["matches"])
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(matches))
	}
}

func TestWriteJSONOrdersMatches(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: sampleMatches(),
		Stats: scan.Stats{
			FilesScanned: 2,
			FilesSkipped: 1,
			Matches:      5,
			DurationMs:   12,
		},
	}

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got jsonOutputResult
	if err := json.Unmarshal(buffer.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}

	if got.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: %d", got.SchemaVersion)
	}
	if len(got.Matches) != 5 {
		t.Fatalf("unexpected match count: %d", len(got.Matches))
	}

	assertFirstMatch(t, got.Matches[0])
	assertWarningOrdering(t, got.Matches[1], got.Matches[2])
	assertInfoMatch(t, got.Matches[3])
	assertLastMatch(t, got.Matches[4])
}

func TestWriteJSONUsesLowerCamelCaseKeys(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{
				Message:   "Hello",
				Severity:  "warning",
				FilePath:  "src/main.go",
				Line:      1,
				Column:    2,
				MatchText: "world",
			},
		},
		Stats: scan.Stats{
			FilesScanned: 1,
			FilesSkipped: 0,
			Matches:      1,
			DurationMs:   5,
		},
	}

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := decodeRawJSON(t, buffer.Bytes())
	match := fetchFirstMatch(t, raw)
	assertLowerCamelCaseMatchKeys(t, match)
	stats := fetchStats(t, raw)
	assertLowerCamelCaseStatsKeys(t, stats)
}

func TestWriteJSONUsesScanRootForAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scanRoot := filepath.Join(root, "fixtures")
	if err := os.MkdirAll(scanRoot, 0o755); err != nil {
		t.Fatalf("failed to create fixtures dir: %v", err)
	}
	filePath := filepath.Join(scanRoot, "sample.txt")
	if err := os.WriteFile(filePath, []byte("token=abc"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	request := scan.Request{
		Roots: []string{scanRoot},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "token=[a-z]+",
				Severity: "error",
				Paths:    []string{"**/*"},
			},
		},
		Include:          []string{"**/*"},
		Exclude:          nil,
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := scan.Run(request)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got jsonOutputResult
	if err := json.Unmarshal(buffer.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}

	expected := fmt.Sprintf("%s:1", filePath)
	if got.Matches[0].AbsolutePath != expected {
		t.Fatalf("unexpected absolutePath: %s", got.Matches[0].AbsolutePath)
	}
}

func TestWriteJSONDoesNotEmitANSIControlSequences(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{
				Message:   "Found token",
				Severity:  "error",
				FilePath:  "src/main.go",
				Line:      3,
				Column:    7,
				MatchText: "token=abc",
			},
		},
		Stats: scan.Stats{FilesScanned: 1, FilesSkipped: 0, Matches: 1, DurationMs: 4},
	}

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNoANSIControlSequences(t, buffer.Bytes())
}

func sampleMatches() []scan.Match {
	return []scan.Match{
		{
			Message:   "Warn msg",
			Severity:  "warning",
			FilePath:  "b/file.go",
			Line:      2,
			Column:    3,
			MatchText: "warn",
		},
		{
			Message:   "Info msg",
			Severity:  "info",
			FilePath:  "a/file.go",
			Line:      10,
			Column:    1,
			MatchText: "info",
		},
		{
			Message:   "Error msg",
			Severity:  "error",
			FilePath:  "a/file.go",
			Line:      2,
			Column:    5,
			MatchText: "error",
		},
		{
			Message:   "Zulu warn",
			Severity:  "warning",
			FilePath:  "a/file.go",
			Line:      2,
			Column:    5,
			MatchText: "zulu",
		},
		{
			Message:   "Alpha warn",
			Severity:  "warning",
			FilePath:  "a/file.go",
			Line:      2,
			Column:    5,
			MatchText: "alpha",
		},
	}
}

func assertFirstMatch(t *testing.T, match jsonOutputMatch) {
	t.Helper()

	if match.FilePath != "a/file.go" {
		t.Fatalf("unexpected first file path: %s", match.FilePath)
	}
	if match.Line != 2 || match.Column != 5 {
		t.Fatalf("unexpected first location: %d:%d", match.Line, match.Column)
	}
	if match.Severity != "error" {
		t.Fatalf("unexpected first severity: %s", match.Severity)
	}
	assertAbsolutePath(t, match.FilePath, match.Line, match.AbsolutePath)
}

func assertWarningOrdering(t *testing.T, first jsonOutputMatch, second jsonOutputMatch) {
	t.Helper()

	if first.Message != "Alpha warn" || second.Message != "Zulu warn" {
		t.Fatalf("unexpected warning ordering: %s, %s", first.Message, second.Message)
	}
	assertAbsolutePath(t, first.FilePath, first.Line, first.AbsolutePath)
	assertAbsolutePath(t, second.FilePath, second.Line, second.AbsolutePath)
}

func assertInfoMatch(t *testing.T, match jsonOutputMatch) {
	t.Helper()

	if match.Message != "Info msg" {
		t.Fatalf("unexpected info ordering: %+v", match)
	}
	assertAbsolutePath(t, match.FilePath, match.Line, match.AbsolutePath)
}

func assertLastMatch(t *testing.T, match jsonOutputMatch) {
	t.Helper()

	if match.FilePath != "b/file.go" {
		t.Fatalf("unexpected last match: %+v", match)
	}
	assertAbsolutePath(t, match.FilePath, match.Line, match.AbsolutePath)
}

func decodeRawJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}

	return raw
}

func fetchFirstMatch(t *testing.T, raw map[string]any) map[string]any {
	t.Helper()

	matches, ok := raw["matches"].([]any)
	if !ok || len(matches) != 1 {
		t.Fatalf("expected one match entry, got %v", raw["matches"])
	}
	match, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected match object, got %T", matches[0])
	}

	return match
}

func assertLowerCamelCaseMatchKeys(t *testing.T, match map[string]any) {
	t.Helper()

	if _, ok := match["message"]; !ok {
		t.Fatalf("expected message key, got %v", match)
	}
	if _, ok := match["Message"]; ok {
		t.Fatalf("expected lowercase message key, got %v", match)
	}
	if _, ok := match["matchText"]; !ok {
		t.Fatalf("expected matchText key, got %v", match)
	}
	if _, ok := match["fileUri"]; ok {
		t.Fatalf("expected no fileUri key, got %v", match)
	}
	if _, ok := match["absolutePath"]; !ok {
		t.Fatalf("expected absolutePath key, got %v", match)
	}
}

func assertAbsolutePath(t *testing.T, filePath string, line int, absolutePath string) {
	t.Helper()

	if absolutePath == "" {
		t.Fatalf("expected absolutePath to be set")
	}
	if filePath == "" {
		t.Fatalf("expected file path to be set")
	}

	expected, err := absolutePathWithLine(filePath, "", line)
	if err != nil {
		t.Fatalf("failed to build absolute path: %v", err)
	}
	if absolutePath != expected {
		t.Fatalf("unexpected absolutePath: %s", absolutePath)
	}
}

func fetchStats(t *testing.T, raw map[string]any) map[string]any {
	t.Helper()

	stats, ok := raw["stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected stats object, got %T", raw["stats"])
	}

	return stats
}

func assertLowerCamelCaseStatsKeys(t *testing.T, stats map[string]any) {
	t.Helper()

	if _, ok := stats["filesScanned"]; !ok {
		t.Fatalf("expected filesScanned key, got %v", stats)
	}
	if _, ok := stats["FilesScanned"]; ok {
		t.Fatalf("expected lowercase filesScanned key, got %v", stats)
	}
}
