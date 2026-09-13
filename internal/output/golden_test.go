//nolint:testpackage
package output

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

func TestGoldenConsoleOutput(t *testing.T) {
	t.Parallel()

	result := goldenSampleResult()

	var buffer bytes.Buffer
	if err := WriteConsole(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertGoldenBytes(t, "console.txt", buffer.Bytes())
}

func TestGoldenConsoleOutputWithColors(t *testing.T) {
	t.Parallel()

	result := goldenSampleResult()

	var buffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: true, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertGoldenBytes(t, "console-color.txt", buffer.Bytes())
}

func TestGoldenJSONOutput(t *testing.T) {
	t.Parallel()

	result := goldenSampleResult()

	var buffer bytes.Buffer
	if err := WriteJSON(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertGoldenBytes(t, "output.json", buffer.Bytes())
}

func TestGoldenSARIFOutput(t *testing.T) {
	t.Parallel()

	result := goldenSarifResult()
	ruleset := goldenSarifRules()

	var buffer bytes.Buffer
	if err := WriteSARIF(result, ruleset, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertGoldenBytes(t, "output.sarif", buffer.Bytes())
}

func goldenSampleResult() scan.Result {
	return scan.Result{
		Matches: []scan.Match{
			{
				Message:  "Warn msg",
				Severity: "warning",
				FilePath: "b/file.go",
				Line:     2,
				Column:   3,
			},
			{
				Message:  "Info msg",
				Severity: "info",
				FilePath: "a/file.go",
				Line:     10,
				Column:   1,
			},
			{
				Message:  "Error msg",
				Severity: "error",
				FilePath: "a/file.go",
				Line:     2,
				Column:   5,
			},
			{
				Message:  "Zulu warn",
				Severity: "warning",
				FilePath: "a/file.go",
				Line:     2,
				Column:   5,
			},
			{
				Message:  "Alpha warn",
				Severity: "warning",
				FilePath: "a/file.go",
				Line:     2,
				Column:   5,
			},
		},
		Stats: scan.Stats{
			FilesScanned: 2,
			FilesSkipped: 1,
			Matches:      5,
			DurationMs:   12,
		},
	}
}

func goldenSarifRules() []rules.Rule {
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

func goldenSarifResult() scan.Result {
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

func assertGoldenBytes(t *testing.T, name string, data []byte) {
	t.Helper()

	// ponytail: normalize checkout dir so goldens survive repo directory renames
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	data = bytes.ReplaceAll(data, []byte(cwd), []byte("<PKG_DIR>"))
	path := goldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}

		return
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	golden = bytes.ReplaceAll(golden, []byte(cwd), []byte("<PKG_DIR>"))
	if !bytes.Equal(golden, data) {
		t.Fatalf("golden mismatch for %s", name)
	}
}

func goldenPath(name string) string {
	return filepath.Join("..", "..", "testdata", "golden", name)
}
