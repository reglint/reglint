//nolint:testpackage
package output

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

func TestWriteConsoleNoMatches(t *testing.T) {
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
	if err := WriteConsole(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "No matches found.\n" +
		"Summary: files=0 skipped=0 matches=0 durationMs=0\n"
	if buffer.String() != expected {
		t.Fatalf("unexpected console output:\n%s", buffer.String())
	}
}

func TestWriteConsoleOrdersAndGroupsMatches(t *testing.T) {
	t.Parallel()

	result := scan.Result{
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

	var buffer bytes.Buffer
	if err := WriteConsole(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertConsoleOutput(t, buffer.String(), expectedGroupedOutput(t))
}

func TestWriteConsoleDeterministicAcrossEquivalentSortKeys(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	rootA := filepath.Join(base, "a-root")
	rootB := filepath.Join(base, "b-root")

	left := scan.Match{
		Message:  "Duplicate msg",
		Severity: "warning",
		FilePath: "src/file.go",
		Root:     rootA,
		Line:     3,
		Column:   7,
	}
	right := left
	right.Root = rootB

	resultAB := scan.Result{
		Matches: []scan.Match{left, right},
		Stats: scan.Stats{
			FilesScanned: 2,
			FilesSkipped: 0,
			Matches:      2,
			DurationMs:   4,
		},
	}
	resultBA := scan.Result{
		Matches: []scan.Match{right, left},
		Stats: scan.Stats{
			FilesScanned: 2,
			FilesSkipped: 0,
			Matches:      2,
			DurationMs:   4,
		},
	}

	var bufferAB bytes.Buffer
	if err := WriteConsole(resultAB, &bufferAB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bufferBA bytes.Buffer
	if err := WriteConsole(resultBA, &bufferBA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bufferAB.String() != bufferBA.String() {
		t.Fatalf(
			"expected deterministic output regardless of input order\nAB:\n%s\nBA:\n%s",
			bufferAB.String(),
			bufferBA.String(),
		)
	}
}

func TestWriteConsoleWithSettingsAppliesANSISeverityColors(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{Message: "Error msg", Severity: "error", FilePath: "a/file.go", Line: 1, Column: 1},
			{Message: "Warn msg", Severity: "warning", FilePath: "a/file.go", Line: 2, Column: 1},
			{Message: "Notice msg", Severity: "notice", FilePath: "a/file.go", Line: 3, Column: 1},
			{Message: "Info msg", Severity: "info", FilePath: "a/file.go", Line: 4, Column: 1},
		},
		Stats: scan.Stats{FilesScanned: 1, Matches: 4},
	}

	var buffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: true, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	consoleOutput := buffer.String()
	expectedLines := []string{
		"- \x1b[31mERROR\x1b[0m 1:1 Error msg",
		"- \x1b[33mWARN\x1b[0m  2:1 Warn msg",
		"- \x1b[36mNOTICE\x1b[0m 3:1 Notice msg",
		"- \x1b[34mINFO\x1b[0m  4:1 Info msg",
	}
	for _, expected := range expectedLines {
		if !strings.Contains(consoleOutput, expected) {
			t.Fatalf("expected %q in console output, got:\n%s", expected, consoleOutput)
		}
	}
}

func TestWriteConsoleWithSettingsResetsANSIColorPerSeverityLabel(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{Message: "Error msg", Severity: "error", FilePath: "a/file.go", Line: 1, Column: 1},
			{Message: "Warn msg", Severity: "warning", FilePath: "a/file.go", Line: 2, Column: 1},
			{Message: "Notice msg", Severity: "notice", FilePath: "a/file.go", Line: 3, Column: 1},
			{Message: "Info msg", Severity: "info", FilePath: "a/file.go", Line: 4, Column: 1},
		},
		Stats: scan.Stats{FilesScanned: 1, Matches: 4},
	}

	var buffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: true, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	consoleOutput := buffer.String()
	if strings.Count(consoleOutput, "\x1b[") != 8 {
		t.Fatalf("expected 8 ANSI sequences (4 open + 4 reset), got output:\n%s", consoleOutput)
	}
	if strings.Count(consoleOutput, "\x1b[0m") != 4 {
		t.Fatalf("expected one reset per severity label, got output:\n%s", consoleOutput)
	}

	tail := strings.Split(consoleOutput, "\nSummary:")[0]
	tail = strings.TrimSpace(tail)
	if strings.Contains(tail, "\n  \x1b[") {
		t.Fatalf("expected absolute path lines to remain uncolored, got:\n%s", consoleOutput)
	}
}

func TestWriteConsoleWithSettingsDisabledMatchesPlainOutput(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{{Message: "Warn msg", Severity: "warning", FilePath: "a/file.go", Line: 2, Column: 3}},
		Stats:   scan.Stats{FilesScanned: 1, Matches: 1},
	}

	var plainBuffer bytes.Buffer
	if err := WriteConsole(result, &plainBuffer); err != nil {
		t.Fatalf("unexpected plain output error: %v", err)
	}

	var disabledBuffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: false, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &disabledBuffer); err != nil {
		t.Fatalf("unexpected disabled output error: %v", err)
	}

	if disabledBuffer.String() != plainBuffer.String() {
		t.Fatalf(
			"expected disabled output to match plain output\nplain:\n%s\ndisabled:\n%s",
			plainBuffer.String(),
			disabledBuffer.String(),
		)
	}
	if strings.Contains(disabledBuffer.String(), "\x1b[") {
		t.Fatalf("expected no ANSI sequences when colors are disabled, got:\n%s", disabledBuffer.String())
	}
}

func TestWriteConsoleWithSettingsConfigDisabledModeHasNoANSI(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{
			{Message: "Error msg", Severity: "error", FilePath: "a/file.go", Line: 1, Column: 1},
			{Message: "Warn msg", Severity: "warning", FilePath: "a/file.go", Line: 2, Column: 1},
			{Message: "Notice msg", Severity: "notice", FilePath: "a/file.go", Line: 3, Column: 1},
			{Message: "Info msg", Severity: "info", FilePath: "a/file.go", Line: 4, Column: 1},
		},
		Stats: scan.Stats{FilesScanned: 1, Matches: 4},
	}

	var buffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: false, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &buffer); err != nil {
		t.Fatalf("unexpected disabled output error: %v", err)
	}

	consoleOutput := buffer.String()
	if strings.Contains(consoleOutput, "\x1b[") {
		t.Fatalf("expected no ANSI sequences in config-disabled mode, got:\n%s", consoleOutput)
	}

	expectedLines := []string{
		"- ERROR 1:1 Error msg",
		"- WARN  2:1 Warn msg",
		"- NOTICE 3:1 Notice msg",
		"- INFO  4:1 Info msg",
	}
	for _, expected := range expectedLines {
		if !strings.Contains(consoleOutput, expected) {
			t.Fatalf("expected %q in config-disabled output, got:\n%s", expected, consoleOutput)
		}
	}
}

func TestWriteConsoleWithSettingsUsesDefaultsWhenSourceMissing(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{{Message: "Error msg", Severity: "error", FilePath: "a/file.go", Line: 1, Column: 1}},
		Stats:   scan.Stats{FilesScanned: 1, Matches: 1},
	}

	var buffer bytes.Buffer
	if err := WriteConsoleWithSettings(result, ConsoleColorSettings{}, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buffer.String(), "\x1b[31mERROR\x1b[0m") {
		t.Fatalf("expected default colorized severity, got:\n%s", buffer.String())
	}
}

func TestWriteConsoleWithSettingsUnknownSeverityStaysPlain(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{{Message: "Custom msg", Severity: "critical", FilePath: "a/file.go", Line: 1, Column: 1}},
		Stats:   scan.Stats{FilesScanned: 1, Matches: 1},
	}

	var buffer bytes.Buffer
	settings := ConsoleColorSettings{Enabled: true, Source: ConsoleColorSourceConfig}
	if err := WriteConsoleWithSettings(result, settings, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	consoleOutput := buffer.String()
	if !strings.Contains(consoleOutput, "- CRITICAL 1:1 Custom msg") {
		t.Fatalf("expected plain unknown severity label, got:\n%s", consoleOutput)
	}
	if strings.Contains(consoleOutput, "\x1b[") {
		t.Fatalf("expected unknown severity to have no ANSI color, got:\n%s", consoleOutput)
	}
}

func TestWriteConsoleReturnsErrorForEmptyFilePath(t *testing.T) {
	t.Parallel()

	result := scan.Result{
		Matches: []scan.Match{{Message: "Error msg", Severity: "error", FilePath: "", Line: 1, Column: 1}},
		Stats:   scan.Stats{FilesScanned: 1, Matches: 1},
	}

	if err := WriteConsole(result, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for empty file path")
	}
}

func TestNormalizeConsoleColorSettings(t *testing.T) {
	t.Parallel()

	defaults := normalizeConsoleColorSettings(ConsoleColorSettings{})
	if !defaults.Enabled {
		t.Fatal("expected default console colors to be enabled")
	}
	if defaults.Source != ConsoleColorSourceDefault {
		t.Fatalf("expected default source, got %q", defaults.Source)
	}

	settings := ConsoleColorSettings{Enabled: false, Source: ConsoleColorSourceEnv}
	normalized := normalizeConsoleColorSettings(settings)
	if normalized != settings {
		t.Fatalf("expected unchanged settings %+v, got %+v", settings, normalized)
	}
}

func TestFormatSeveritySegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		severity      string
		colorsEnabled bool
		want          string
	}{
		{name: "plain error", severity: "error", colorsEnabled: false, want: "ERROR"},
		{name: "colorized warn with padding", severity: "warning", colorsEnabled: true, want: "\x1b[33mWARN\x1b[0m "},
		{name: "colorized notice without padding", severity: "notice", colorsEnabled: true, want: "\x1b[36mNOTICE\x1b[0m"},
		{name: "unknown severity plain", severity: "custom", colorsEnabled: true, want: "CUSTOM"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := formatSeveritySegment(testCase.severity, testCase.colorsEnabled)
			if got != testCase.want {
				t.Fatalf("unexpected formatted severity: got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestAbsolutePathWithLineRequiresFilePath(t *testing.T) {
	t.Parallel()

	if _, err := absolutePathWithLine("", "", 1); err == nil {
		t.Fatal("expected error for empty file path")
	}
}

func TestConsoleFormatterWriteUsesColorSettings(t *testing.T) {
	t.Parallel()

	formatter := ConsoleFormatter{ColorSettings: ConsoleColorSettings{Enabled: true, Source: ConsoleColorSourceConfig}}
	if formatter.Name() != "console" {
		t.Fatalf("unexpected formatter name: %s", formatter.Name())
	}

	result := scan.Result{
		Matches: []scan.Match{{Message: "Error msg", Severity: "error", FilePath: "a/file.go", Line: 1, Column: 1}},
		Stats:   scan.Stats{FilesScanned: 1, Matches: 1},
	}

	var buffer bytes.Buffer
	if err := formatter.Write(result, &buffer); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if !strings.Contains(buffer.String(), "\x1b[31mERROR\x1b[0m") {
		t.Fatalf("expected colorized formatter output, got:\n%s", buffer.String())
	}
}

func TestFormatConsoleMatchLine(t *testing.T) {
	t.Parallel()

	match := scan.Match{
		Message:  "Error msg",
		Severity: "error",
		FilePath: "a/file.go",
		Line:     2,
		Column:   5,
	}

	line, err := formatConsoleMatchLine(match)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absPath, err := expectedAbsolutePathLine(match.FilePath, match.Line)
	if err != nil {
		t.Fatalf("failed to build absolute path: %v", err)
	}

	expected := "- ERROR 2:5 Error msg\n" +
		"  " + absPath + "\n"
	if line != expected {
		t.Fatalf("unexpected match line: %s", line)
	}
}

func TestWriteConsoleUsesScanRootForAbsolutePath(t *testing.T) {
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
	if err := WriteConsole(result, &buffer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLine := fmt.Sprintf("%s:1", filePath)
	if !strings.Contains(buffer.String(), expectedLine) {
		t.Fatalf("expected absolute path %s in output, got:\n%s", expectedLine, buffer.String())
	}
}

func TestFormatConsoleMatchLineReturnsErrorWhenCwdMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("current directory removal is restricted on windows")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unexpected getwd error: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("unexpected chdir restore error: %v", err)
		}
	}()

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("unexpected chdir error: %v", err)
	}
	if err := os.RemoveAll(tempDir); err != nil {
		t.Fatalf("unexpected remove error: %v", err)
	}

	_, err = formatConsoleMatchLine(scan.Match{FilePath: "relative/file.go", Line: 1})
	if err == nil {
		t.Fatalf("expected error with missing cwd")
	}

	_, err = formatConsoleMatchLineWithColor(scan.Match{
		FilePath: "relative/file.go",
		Root:     "relative-root",
		Line:     1,
	}, false)
	if err == nil {
		t.Fatalf("expected error with missing cwd and non-empty root")
	}
}

func expectedGroupedOutput(t *testing.T) string {
	t.Helper()

	fileA2 := expectedAbsolutePathLineHelper(t, "a/file.go", 2)
	fileA10 := expectedAbsolutePathLineHelper(t, "a/file.go", 10)
	fileB2 := expectedAbsolutePathLineHelper(t, "b/file.go", 2)

	return "a/file.go\n" +
		"- ERROR 2:5 Error msg\n" +
		"  " + fileA2 + "\n\n" +
		"- WARN  2:5 Alpha warn\n" +
		"  " + fileA2 + "\n\n" +
		"- WARN  2:5 Zulu warn\n" +
		"  " + fileA2 + "\n\n" +
		"- INFO  10:1 Info msg\n" +
		"  " + fileA10 + "\n\n" +
		"b/file.go\n" +
		"- WARN  2:3 Warn msg\n" +
		"  " + fileB2 + "\n\n" +
		"Summary: files=2 skipped=1 matches=5 durationMs=12\n"
}

func assertConsoleOutput(t *testing.T, got, want string) {
	t.Helper()

	if got != want {
		t.Fatalf("unexpected console output:\n%s", got)
	}
}

func expectedAbsolutePathLine(filePath string, line int) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", absPath, line), nil
}

func expectedAbsolutePathLineHelper(t *testing.T, filePath string, line int) string {
	t.Helper()

	absPath, err := expectedAbsolutePathLine(filePath, line)
	if err != nil {
		t.Fatalf("failed to build absolute path: %v", err)
	}

	return absPath
}

func TestSeverityRankKnownValues(t *testing.T) {
	t.Parallel()

	values := []string{"error", "warning", "notice", "info"}
	for _, value := range values {
		if severityRank(value) == severityRankUnknown {
			t.Fatalf("expected severity %s to have known rank", value)
		}
	}
}

func TestSeverityRankUnknownValue(t *testing.T) {
	t.Parallel()

	if severityRank("custom") != severityRankUnknown {
		t.Fatalf("expected unknown severity rank")
	}
}

func TestSeverityLabelUnknownUppercase(t *testing.T) {
	t.Parallel()

	if severityLabel("custom") != "CUSTOM" {
		t.Fatalf("unexpected label for custom severity")
	}
}
