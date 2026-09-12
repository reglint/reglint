package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iyaki/reglint/internal/cli"
	"github.com/iyaki/reglint/internal/scan"
)

func TestRunShowsHelpWhenNoArgs(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	if !strings.Contains(output.String(), "Usage:") {
		t.Fatalf("expected usage help, got %q", output.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"bogus"}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	if output.String() != "Unknown command: bogus\n" {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestRunShowsRootHelpForLongFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"--help"}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if output.String() != expectedRootHelpOutput() {
		t.Fatalf("unexpected root help output: %q", output.String())
	}
}

func TestRunShowsRootHelpForShortFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"-h"}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if output.String() != expectedRootHelpOutput() {
		t.Fatalf("unexpected root help output: %q", output.String())
	}
}

func TestRunShowsAnalyzeHelpForLongFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"analyze", "--help"}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if output.String() != expectedAnalyzeHelpOutput() {
		t.Fatalf("unexpected analyze help output: %q", output.String())
	}
}

func TestRunShowsAnalyseHelpForShortFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"analyse", "-h"}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if output.String() != expectedAnalyzeHelpOutput() {
		t.Fatalf("unexpected analyse help output: %q", output.String())
	}
}

func TestRunUnknownCommandWithHelpFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"bogus", "--help"}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if output.String() != "Unknown command: bogus\n" {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestRunRoutesAnalyze(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"analyze"}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1 for missing config, got %d", code)
	}
	if !strings.Contains(output.String(), "config file not found: reglint-rules.yaml") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestRunRoutesAnalyseAlias(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := run([]string{"analyse"}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1 for missing config, got %d", code)
	}
	if !strings.Contains(output.String(), "config file not found: reglint-rules.yaml") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestRunRoutesInit(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "rules.yaml")
	var output bytes.Buffer
	code := run([]string{"init", "--out", outputPath}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	expectedMessage := "Wrote default config to " + outputPath + "\n"
	if output.String() != expectedMessage {
		t.Fatalf("unexpected output: %q", output.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	contents := string(data)
	if !strings.Contains(contents, "rules:") {
		t.Fatalf("expected rules section, got %q", contents)
	}
	if !strings.Contains(contents, "Avoid hardcoded token") {
		t.Fatalf("expected default rule, got %q", contents)
	}
}

func TestRunAnalyzeWritesJSONToStdout(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{"analyze", "--config", configPath, "--format", "json", rootDir}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var got jsonResult
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: %d", got.SchemaVersion)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}
	match := got.Matches[0]
	if match.FilePath != "sample.txt" {
		t.Fatalf("unexpected match file: %s", match.FilePath)
	}
	if match.Severity != "error" {
		t.Fatalf("unexpected match severity: %s", match.Severity)
	}
	if got.Stats.Matches != 1 {
		t.Fatalf("unexpected stats matches: %d", got.Stats.Matches)
	}
}

func TestRunAnalyzeWritesJSONFileForMultiFormat(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")
	jsonPath := filepath.Join(t.TempDir(), "scan.json")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "console,json",
		"--out-json", jsonPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "Summary:") {
		t.Fatalf("expected console output summary, got %q", output.String())
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read json output: %v", err)
	}
	var got jsonResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to parse json file: %v", err)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}
}

func TestRunAnalyzeConsoleUsesANSIColorsByDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "console",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "\x1b[31mERROR\x1b[0m") {
		t.Fatalf("expected ANSI-colored error label, got %q", output.String())
	}
}

func TestRunAnalyzeConsoleDisablesANSIWithNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "console",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected NO_COLOR to disable ANSI output, got %q", output.String())
	}
}

func TestRunAnalyzeConsoleDisablesANSIWhenConfigDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(t, configDir, "consoleColorsEnabled: false\n")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "console",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected config to disable ANSI output, got %q", output.String())
	}
}

func TestRunAnalyzeExitCodeFailOn(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--fail-on", "warning",
		rootDir,
	}, &output)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunAnalyzeUsesTestdataExampleConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "testdata", "rules", "example.yaml")
	fixturesPath := filepath.Join("..", "..", "testdata", "fixtures")

	var output bytes.Buffer
	code := run([]string{"analyze", "--config", configPath, fixturesPath}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "Found token") {
		t.Fatalf("expected output to contain match message, got %q", output.String())
	}
}

func TestRunAnalyzeUsesTestdataFailConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "testdata", "rules", "fail.yaml")
	fixturesPath := filepath.Join("..", "..", "testdata", "fixtures")

	var output bytes.Buffer
	code := run([]string{"analyze", "--config", configPath, fixturesPath}, &output)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(output.String(), "Found token") {
		t.Fatalf("expected output to contain match message, got %q", output.String())
	}
}

func TestRunAnalyzeUsesBaselineFixtureForCompareMode(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "testdata", "rules", "fail.yaml")
	baselinePath := filepath.Join("..", "..", "testdata", "baseline", "valid-equal.json")
	fixturesPath := filepath.Join("..", "..", "testdata", "fixtures")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		fixturesPath,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected suppressed output, got %q", output.String())
	}
}

func TestRunAnalyzeBaselineIncreaseReportsOnlyExcessRegressions(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc\ntoken=abc\n")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	baselinePayload := []byte(`{
		"schemaVersion": 1,
		"entries": [
			{
				"filePath": "sample.txt",
				"message": "Found token token=abc",
				"count": 1
			}
		]
	}`)
	if err := os.WriteFile(baselinePath, baselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--fail-on", "warning",
		rootDir,
	}, &output)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if got := strings.Count(output.String(), "Found token token=abc"); got != 1 {
		t.Fatalf("expected one regression output, got %d in %q", got, output.String())
	}
	if !strings.Contains(output.String(), "matches=1") {
		t.Fatalf("expected one regression in summary, got %q", output.String())
	}
}

func TestRunAnalyzeBaselineDecreaseDoesNotFailOnSuppressedKey(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc\n")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	baselinePayload := []byte(`{
		"schemaVersion": 1,
		"entries": [
			{
				"filePath": "sample.txt",
				"message": "Found token token=abc",
				"count": 2
			}
		]
	}`)
	if err := os.WriteFile(baselinePath, baselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--fail-on", "warning",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected suppressed output, got %q", output.String())
	}
	if !strings.Contains(output.String(), "matches=0") {
		t.Fatalf("expected zero regressions in summary, got %q", output.String())
	}
}

func TestRunAnalyzeUsesRuleSetBaselineFixture(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "testdata", "rules", "baseline.yaml")
	fixturesPath := filepath.Join("..", "..", "testdata", "fixtures")

	var output bytes.Buffer
	code := run([]string{"analyze", "--config", configPath, fixturesPath}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected suppressed output, got %q", output.String())
	}
}

func TestRunAnalyzeCLIBaselineOverridesRuleSetBaseline(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(t, configDir, "baseline: \"ruleset-baseline.json\"\n")

	ruleSetBaselinePath := filepath.Join(configDir, "ruleset-baseline.json")
	if err := os.WriteFile(ruleSetBaselinePath, []byte(`{"schemaVersion":1,"entries":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write ruleset baseline fixture: %v", err)
	}

	cliBaselinePath := filepath.Join(t.TempDir(), "cli-baseline.json")
	cliBaselinePayload := []byte(`{
		"schemaVersion": 1,
		"entries": [
			{
				"filePath": "sample.txt",
				"message": "Found token token=abc",
				"count": 1
			}
		]
	}`)
	if err := os.WriteFile(cliBaselinePath, cliBaselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write cli baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", cliBaselinePath,
		"--fail-on", "warning",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected CLI baseline suppression to apply, got %q", output.String())
	}
	if strings.Contains(output.String(), "Found token token=abc") {
		t.Fatalf("expected ruleset baseline to be overridden, got %q", output.String())
	}
}

func TestRunAnalyzeRejectsInvalidBaselineJSONWithSingleErrorMessage(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(baselinePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("failed to write invalid baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		rootDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "parse baseline") {
		t.Fatalf("expected baseline parse error, got %q", output.String())
	}
	assertSingleErrorMessage(t, output.String())
}

func TestRunAnalyzeRejectsInvalidBaselineSchemaVersionWithSingleErrorMessage(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "invalid-schema.json")
	baselinePayload := []byte(`{"schemaVersion":2,"entries":[]}`)
	if err := os.WriteFile(baselinePath, baselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write invalid baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		rootDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "baseline schemaVersion must be 1") {
		t.Fatalf("expected baseline schema validation error, got %q", output.String())
	}
	assertSingleErrorMessage(t, output.String())
}

func TestRunAnalyzeRejectsInvalidBaselineFixture(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "testdata", "rules", "fail.yaml")
	baselinePath := filepath.Join("..", "..", "testdata", "baseline", "invalid-duplicate-keys.json")
	fixturesPath := filepath.Join("..", "..", "testdata", "fixtures")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		fixturesPath,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "duplicate baseline entry") {
		t.Fatalf("expected baseline validation error, got %q", output.String())
	}
	assertSingleErrorMessage(t, output.String())
}

func TestRunAnalyzeWriteBaselineRequiresEffectiveBaselinePath(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--write-baseline",
		rootDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "--write-baseline requires an effective baseline path") {
		t.Fatalf("expected missing baseline path error, got %q", output.String())
	}
	assertSingleErrorMessage(t, output.String())
}

func TestRunAnalyzeWriteBaselineIgnoresExistingBaselineContent(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("failed to write invalid baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--write-baseline",
		"--format", "json",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	gotResult := decodeJSONResult(t, output.Bytes())
	if len(gotResult.Matches) != 1 {
		t.Fatalf("expected 1 match in write mode output, got %d", len(gotResult.Matches))
	}

	baselineDoc := readBaselineResult(t, baselinePath)
	if baselineDoc.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", baselineDoc.SchemaVersion)
	}
	if len(baselineDoc.Entries) != 1 {
		t.Fatalf("expected 1 baseline entry, got %d", len(baselineDoc.Entries))
	}
	assertBaselineEntry(t, baselineDoc.Entries[0], "sample.txt", "Found token token=abc", 1)
}

func TestRunAnalyzeWriteBaselineExitsZeroWithFailOnMatches(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--write-baseline",
		"--fail-on", "warning",
		"--format", "json",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	gotResult := decodeJSONResult(t, output.Bytes())
	if len(gotResult.Matches) != 1 {
		t.Fatalf("expected full findings in write mode, got %d", len(gotResult.Matches))
	}
	if gotResult.Stats.Matches != 1 {
		t.Fatalf("expected stats matches=1, got %d", gotResult.Stats.Matches)
	}
}

func TestRunAnalyzeBaselineCompareJSONOutputRemainsANSIFreeAndSchemaStable(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	baselinePayload := []byte(`{
		"schemaVersion": 1,
		"entries": [
			{
				"filePath": "sample.txt",
				"message": "Found token token=abc",
				"count": 1
			}
		]
	}`)
	if err := os.WriteFile(baselinePath, baselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--format", "json",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected ANSI-free JSON output, got %q", output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if got.SchemaVersion != 1 {
		t.Fatalf("expected schemaVersion 1, got %d", got.SchemaVersion)
	}
	if len(got.Matches) != 0 {
		t.Fatalf("expected baseline-filtered JSON matches to be empty, got %d", len(got.Matches))
	}
	if got.Stats.Matches != 0 {
		t.Fatalf("expected baseline-filtered JSON stats.matches to be 0, got %d", got.Stats.Matches)
	}
}

func TestRunAnalyzeBaselineCompareSARIFOutputRemainsANSIFreeAndSchemaStable(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	baselinePayload := []byte(`{
		"schemaVersion": 1,
		"entries": [
			{
				"filePath": "sample.txt",
				"message": "Found token token=abc",
				"count": 1
			}
		]
	}`)
	if err := os.WriteFile(baselinePath, baselinePayload, 0o600); err != nil {
		t.Fatalf("failed to write baseline fixture: %v", err)
	}

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--baseline", baselinePath,
		"--format", "sarif",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected ANSI-free SARIF output, got %q", output.String())
	}

	got := decodeSARIFLog(t, output.Bytes())
	if got.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Fatalf("unexpected SARIF schema: %q", got.Schema)
	}
	if got.Version != "2.1.0" {
		t.Fatalf("unexpected SARIF version: %q", got.Version)
	}
	if len(got.Runs) != 1 {
		t.Fatalf("expected one SARIF run, got %d", len(got.Runs))
	}
	if got.Runs[0].ColumnKind != "unicodeCodePoints" {
		t.Fatalf("unexpected columnKind: %q", got.Runs[0].ColumnKind)
	}
	if len(got.Runs[0].Results) != 0 {
		t.Fatalf("expected baseline-filtered SARIF results to be empty, got %d", len(got.Runs[0].Results))
	}
}

func TestRunAnalyzeGitModeOffDoesNotRequireGit(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "off",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}
	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with git-mode=off, got %d", len(got.Matches))
	}
}

func TestRunAnalyzeGitModeOffAppliesGitignoreByDefault(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, ".gitignore", "sample.txt\n")
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "off",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}
	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 0 {
		t.Fatalf("expected 0 matches with git-mode=off when .gitignore excludes sample.txt, got %d", len(got.Matches))
	}
}

func TestRunAnalyzeGitModeOffNoGitignoreFlagDisablesConfiguredGitignore(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, ".gitignore", "sample.txt\n")
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(t, configDir, "ignoreFiles:\n  - \".gitignore\"\n")

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "off",
		"--no-gitignore",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}
	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with --no-gitignore in git-mode=off, got %d", len(got.Matches))
	}
}

func TestRunAnalyzeGitModeOffNoIgnoreFilesFlagDisablesAllIgnoreProcessing(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, ".gitignore", "sample.txt\n")
	writeFixture(t, rootDir, ".ignore", "sample.txt\n")
	writeFixture(t, rootDir, ".reglintignore", "sample.txt\n")
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "off",
		"--no-ignore-files",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}
	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with --no-ignore-files in git-mode=off, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "sample.txt" {
		t.Fatalf("expected sample.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeOffRuleSetGitignoreDisabledDisablesConfiguredGitignore(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, ".gitignore", "sample.txt\n")
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfigWithPreamble(
		t,
		configDir,
		gitignoreDisabledWithConfiguredIgnorePreamble,
	)

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "off",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}
	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with git.gitignoreEnabled=false in git-mode=off, got %d", len(got.Matches))
	}
}

func TestRunAnalyzeGitModeStagedNoGitignoreFlagDisablesConfiguredGitignore(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "generated"), os.ModePerm); err != nil {
		t.Fatalf("failed to create generated directory: %v", err)
	}
	writeFixture(t, repoDir, ".gitignore", "generated/**\n")
	writeFixture(t, repoDir, "generated/keep.txt", "token=abc\n")
	runGit(t, repoDir, "add", "-f", "generated/keep.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfigWithPreamble(t, configDir, "ignoreFiles:\n  - \".gitignore\"\n")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		"--no-gitignore",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with --no-gitignore in git-mode=staged, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "generated/keep.txt" {
		t.Fatalf("expected generated/keep.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeStagedNoIgnoreFilesFlagDisablesAllIgnoreProcessing(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "generated"), os.ModePerm); err != nil {
		t.Fatalf("failed to create generated directory: %v", err)
	}
	writeFixture(t, repoDir, ".gitignore", "generated/**\n")
	writeFixture(t, repoDir, ".ignore", "generated/**\n")
	writeFixture(t, repoDir, ".reglintignore", "generated/**\n")
	writeFixture(t, repoDir, "generated/keep.txt", "token=abc\n")
	runGit(t, repoDir, "add", "-f", "generated/keep.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		"--no-ignore-files",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with --no-ignore-files in git-mode=staged, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "generated/keep.txt" {
		t.Fatalf("expected generated/keep.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeStagedRuleSetGitignoreDisabledDisablesConfiguredGitignore(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "generated"), os.ModePerm); err != nil {
		t.Fatalf("failed to create generated directory: %v", err)
	}
	writeFixture(t, repoDir, ".gitignore", "generated/**\n")
	writeFixture(t, repoDir, "generated/keep.txt", "token=abc\n")
	runGit(t, repoDir, "add", "-f", "generated/keep.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfigWithPreamble(
		t,
		configDir,
		gitignoreDisabledWithConfiguredIgnorePreamble,
	)

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match with git.gitignoreEnabled=false in git-mode=staged, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "generated/keep.txt" {
		t.Fatalf("expected generated/keep.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeStagedWithoutGitBinaryReturnsError(t *testing.T) {
	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	t.Setenv("PATH", "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "git mode staged requires git executable") {
		t.Fatalf("expected missing git executable error, got %q", output.String())
	}
}

func TestRunAnalyzeGitModeStagedOutsideRepositoryReturnsError(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	rootDir := t.TempDir()
	configDir := t.TempDir()
	writeFixture(t, rootDir, "sample.txt", "token=abc")
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "git mode staged requires a git repository") {
		t.Fatalf("expected non-repo git error, got %q", output.String())
	}
}

func TestRunAnalyzeGitModeStagedScansOnlyStagedFiles(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "staged.txt", "token=aaa\n")
	writeFixture(t, repoDir, "unstaged.txt", "token=bbb\n")
	runGit(t, repoDir, "add", "staged.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match for staged-only scan, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "staged.txt" {
		t.Fatalf("expected staged.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeDiffScansOnlyChangedFiles(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "changed.txt", "clean\n")
	writeFixture(t, repoDir, "unchanged.txt", "token=abc\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	writeFixture(t, repoDir, "changed.txt", "token=xyz\n")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "diff",
		"--git-diff", "HEAD",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match for diff-only scan, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "changed.txt" {
		t.Fatalf("expected changed.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitDiffFlagImpliesDiffModeAndScopesCandidates(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "changed.txt", "clean\n")
	writeFixture(t, repoDir, "unchanged.txt", "token=abc\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	writeFixture(t, repoDir, "changed.txt", "token=xyz\n")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-diff", "HEAD",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match for implied diff-only scan, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "changed.txt" {
		t.Fatalf("expected changed.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeDiffInvalidTargetReturnsError(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	writeFixture(t, repoDir, "sample.txt", "token=abc\n")
	runGit(t, repoDir, "add", "sample.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--git-mode", "diff",
		"--git-diff", "DOES_NOT_EXIST",
		repoDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "git mode diff failed to resolve changed files for target \"DOES_NOT_EXIST\"") {
		t.Fatalf("expected invalid diff target error, got %q", output.String())
	}
}

func TestRunAnalyzeGitDiffFlagInvalidTargetReturnsError(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	writeFixture(t, repoDir, "sample.txt", "token=abc\n")
	runGit(t, repoDir, "add", "sample.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--git-diff", "DOES_NOT_EXIST",
		repoDir,
	}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "git mode diff failed to resolve changed files for target \"DOES_NOT_EXIST\"") {
		t.Fatalf("expected invalid diff target error, got %q", output.String())
	}
}

func TestRunAnalyzeGitModeStagedAddedLinesOnlyReportsOnlyAddedLines(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "sample.txt", "token=old\nclean\n")
	runGit(t, repoDir, "add", "sample.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	writeFixture(t, repoDir, "sample.txt", "token=old\nclean\ntoken=new\n")
	runGit(t, repoDir, "add", "sample.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		"--git-added-lines-only",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 added-line match, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "sample.txt" {
		t.Fatalf("expected sample.txt match, got %q", got.Matches[0].FilePath)
	}
	if got.Matches[0].Line != 3 {
		t.Fatalf("expected added-line match on line 3, got line %d", got.Matches[0].Line)
	}
}

func TestRunAnalyzeGitModeStagedIgnoreConflictPrefersIgnoreOverGitignore(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "generated"), os.ModePerm); err != nil {
		t.Fatalf("failed to create generated directory: %v", err)
	}
	writeFixture(t, repoDir, ".gitignore", "generated/**\n")
	writeFixture(t, repoDir, ".ignore", "!generated/keep.txt\n")
	writeFixture(t, repoDir, "generated/keep.txt", "token=abc\n")
	runGit(t, repoDir, "add", "-f", "generated/keep.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match when .ignore overrides .gitignore, got %d", len(got.Matches))
	}
	if got.Matches[0].FilePath != "generated/keep.txt" {
		t.Fatalf("expected generated/keep.txt match, got %q", got.Matches[0].FilePath)
	}
}

func TestRunAnalyzeGitModeStagedIgnoreConflictPrefersReglintignoreOverIgnoreAndGitignore(t *testing.T) {
	t.Parallel()
	ensureGitAvailable(t)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "generated"), os.ModePerm); err != nil {
		t.Fatalf("failed to create generated directory: %v", err)
	}
	writeFixture(t, repoDir, ".gitignore", "generated/**\n")
	writeFixture(t, repoDir, ".ignore", "!generated/keep.txt\n")
	writeFixture(t, repoDir, ".reglintignore", "generated/keep.txt\n")
	writeFixture(t, repoDir, "generated/keep.txt", "token=abc\n")
	runGit(t, repoDir, "add", "-f", "generated/keep.txt")

	configDir := t.TempDir()
	configPath := writeRuleConfig(t, configDir, "")

	var output bytes.Buffer
	code := run([]string{
		"analyze",
		"--config", configPath,
		"--format", "json",
		"--git-mode", "staged",
		repoDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output.String())
	}

	got := decodeJSONResult(t, output.Bytes())
	if len(got.Matches) != 0 {
		t.Fatalf("expected 0 matches when .reglintignore overrides .ignore and .gitignore, got %d", len(got.Matches))
	}
}

func TestRunUsesProvidedOutputWriter(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{}, map[string]cli.Handler{}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "Usage:") {
		t.Fatalf("expected usage help, got %q", output.String())
	}
}

func TestMainExitsWithRunCode(t *testing.T) {
	var output bytes.Buffer
	var exitCode int
	var exited bool

	originalArgs := args
	originalOutput := outputWriter
	originalExit := exitFunc
	defer func() {
		args = originalArgs
		outputWriter = originalOutput
		exitFunc = originalExit
	}()

	args = []string{"reglint"}
	outputWriter = &output
	exitFunc = func(code int) {
		exited = true
		exitCode = code
	}

	main()

	if !exited {
		t.Fatal("expected main to call exit")
	}
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(output.String(), "Usage:") {
		t.Fatalf("expected usage help, got %q", output.String())
	}
}

type jsonResult struct {
	SchemaVersion int          `json:"schemaVersion"`
	Matches       []scan.Match `json:"matches"`
	Stats         scan.Stats   `json:"stats"`
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	ColumnKind string        `json:"columnKind"`
	Results    []sarifResult `json:"results"`
}

type sarifResult struct {
	RuleID string `json:"ruleId"`
	Level  string `json:"level"`
}

type baselineResult struct {
	SchemaVersion int             `json:"schemaVersion"`
	Entries       []baselineEntry `json:"entries"`
}

type baselineEntry struct {
	FilePath string `json:"filePath"`
	Message  string `json:"message"`
	Count    int    `json:"count"`
}

func decodeJSONResult(t *testing.T, data []byte) jsonResult {
	t.Helper()

	var result jsonResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}

	return result
}

func decodeSARIFLog(t *testing.T, data []byte) sarifLog {
	t.Helper()

	var log sarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("failed to parse sarif output: %v", err)
	}

	return log
}

func readBaselineResult(t *testing.T, path string) baselineResult {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read baseline output: %v", err)
	}

	var doc baselineResult
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("failed to parse generated baseline: %v", err)
	}

	return doc
}

func assertBaselineEntry(t *testing.T, got baselineEntry, filePath, message string, count int) {
	t.Helper()

	if got.FilePath != filePath || got.Message != message || got.Count != count {
		t.Fatalf("unexpected baseline entry: %+v", got)
	}
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	return path
}

func writeRuleConfig(t *testing.T, dir, failOn string) string {
	t.Helper()

	preamble := ""
	if failOn != "" {
		preamble = "failOn: \"" + failOn + "\"\n"
	}

	return writeRuleConfigWithPreamble(t, dir, preamble)
}

const gitignoreDisabledWithConfiguredIgnorePreamble = "git:\n" +
	"  gitignoreEnabled: false\n" +
	"ignoreFiles:\n" +
	"  - \".gitignore\"\n"

func writeRuleConfigWithPreamble(t *testing.T, dir, preamble string) string {
	t.Helper()

	config := preamble +
		"rules:\n" +
		"  - message: \"Found token $0\"\n" +
		"    regex: \"token=[a-z]+\"\n" +
		"    severity: \"error\"\n"
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return path
}

func ensureGitAvailable(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable is required for this test")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "RegLint Test")
	runGit(t, dir, "config", "user.email", "reglint-test@example.com")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v; output: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}

	return string(output)
}

func assertSingleErrorMessage(t *testing.T, got string) {
	t.Helper()

	trimmed := strings.TrimSuffix(got, "\n")
	if trimmed == "" {
		t.Fatalf("expected error output, got %q", got)
	}
	if strings.Contains(trimmed, "\n") {
		t.Fatalf("expected a single-line error message, got %q", got)
	}
}

func expectedRootHelpOutput() string {
	return "Usage:\n" +
		"  reglint <command> [flags]\n" +
		"\n" +
		"Commands:\n" +
		"  analyze (alias: analyse)\n" +
		"  init\n" +
		"  version\n" +
		"\n" +
		"Flags:\n" +
		"  -h, --help bool (default false)  Print help and exit.\n"
}

func expectedAnalyzeHelpOutput() string {
	return "Usage:\n" +
		"  reglint analyze [flags] [path ...]\n" +
		"  reglint analyse [flags] [path ...]\n" +
		"\n" +
		"Flags:\n" +
		"  -h, --help bool (default false)  Print help and exit.\n" +
		"  -c, --config string (default reglint-rules.yaml)  Path to YAML rules config file.\n" +
		"  -f, --format string (default console)  Comma-separated list of formats.\n" +
		"      --out-json string (default none)  Output path for JSON results.\n" +
		"      --out-sarif string (default none)  Output path for SARIF results.\n" +
		"      --include string (default none)  Repeatable include glob.\n" +
		"      --exclude string (default none)  Repeatable exclude glob.\n" +
		"      --concurrency int (default GOMAXPROCS)  Worker count.\n" +
		"      --max-file-size int (default 5242880)  Maximum file size in bytes.\n" +
		"      --fail-on string (default none)  Fail if matches at or above severity.\n" +
		"      --baseline string (default none)  Baseline JSON path for suppression.\n" +
		"      --write-baseline bool (default false)  Generate/regenerate baseline from findings.\n" +
		"      --git-mode string (default off)  Select Git mode: off, staged, or diff.\n" +
		"      --git-diff string (default none)  Diff target/range for --git-mode=diff.\n" +
		"      --git-added-lines-only bool (default false)  Restrict matches to added lines in Git mode.\n" +
		"      --no-gitignore bool (default false)  Disable .gitignore filtering for this run.\n" +
		"      --no-ignore-files bool (default false)  Disable ignore file loading and matching.\n"
}
