//nolint:testpackage
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/reglint/reglint/internal/baseline"
	"github.com/reglint/reglint/internal/git"
	"github.com/reglint/reglint/internal/output"
	"github.com/reglint/reglint/internal/rules"
)

var cwdMutex sync.Mutex

func TestHandleAnalyzeMissingConfig(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	var output bytes.Buffer
	code := HandleAnalyze([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "config file not found") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeSurfaceRenderErrors(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	config := "rules:\n  - message: 'hello'\n    regex: 'world'\n"
	configPath := writeConfig(t, config)

	var output bytes.Buffer
	code := HandleAnalyze([]string{"--config", configPath, "--format", "json", "--out-json", t.TempDir()}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "output path is a directory") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeSurfaceRegistryErrors(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	currentRegistry := outputRegistry
	outputRegistry = func([]rules.Rule, output.ConsoleColorSettings) (*output.Registry, error) {
		return nil, errors.New("registry failed")
	}
	t.Cleanup(func() {
		outputRegistry = currentRegistry
	})

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "registry failed") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeFailOnThreshold(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--fail-on", "warning",
		rootDir,
	}, &output)

	if code != exitCodeFailOn {
		t.Fatalf("expected exit code %d, got %d", exitCodeFailOn, code)
	}
	if !strings.Contains(output.String(), "Summary:") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeBaselineSuppressionAffectsFailOn(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, sampleConfig())
	baselinePath := writeBaseline(t, baseline.Document{
		SchemaVersion: 1,
		Entries: []baseline.Entry{
			{FilePath: "sample.txt", Message: "Found token token=abc", Count: 1},
		},
	})

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--baseline", baselinePath,
		"--fail-on", "error",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected suppression output, got %q", output.String())
	}
}

func TestHandleAnalyzeBaselineCompareReportsOnlyRegressions(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc\ntoken=abc\n")
	configPath := writeConfig(t, sampleConfig())
	baselinePath := writeBaseline(t, baseline.Document{
		SchemaVersion: 1,
		Entries: []baseline.Entry{
			{FilePath: "sample.txt", Message: "Found token token=abc", Count: 1},
		},
	})

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--baseline", baselinePath,
		"--fail-on", "error",
		rootDir,
	}, &output)

	if code != exitCodeFailOn {
		t.Fatalf("expected exit code %d, got %d", exitCodeFailOn, code)
	}
	if got := strings.Count(output.String(), "Found token token=abc"); got != 1 {
		t.Fatalf("expected one regression match, got %d in output %q", got, output.String())
	}
	if !strings.Contains(output.String(), "matches=1") {
		t.Fatalf("expected summary to report one regression, got %q", output.String())
	}
}

func TestHandleAnalyzeWriteBaselineIgnoresExistingContentAndReturnsZero(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc\ntoken=abc\n")
	configPath := writeConfig(t, sampleConfig())

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, []byte("not-json"), defaultFileMode); err != nil {
		t.Fatalf("failed to seed baseline file: %v", err)
	}

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--baseline", baselinePath,
		"--write-baseline",
		"--fail-on", "error",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("expected full findings in write mode, got %q", output.String())
	}

	document := readBaseline(t, baselinePath)
	if document.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", document.SchemaVersion)
	}
	if len(document.Entries) != 1 {
		t.Fatalf("expected one baseline entry, got %d", len(document.Entries))
	}
	if document.Entries[0].FilePath != "sample.txt" {
		t.Fatalf("expected baseline filePath sample.txt, got %q", document.Entries[0].FilePath)
	}
	if document.Entries[0].Message != "Found token token=abc" {
		t.Fatalf("unexpected baseline message: %q", document.Entries[0].Message)
	}
	if document.Entries[0].Count != 2 {
		t.Fatalf("expected baseline count 2, got %d", document.Entries[0].Count)
	}
}

func TestExitCodeFailOnConstant(t *testing.T) {
	t.Parallel()

	if exitCodeFailOn != 2 {
		t.Fatalf("expected exitCodeFailOn to be 2, got %d", exitCodeFailOn)
	}
}

func TestHandleAnalyzeNoMatches(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeReturnsZeroWhenFailOnUnset(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "Summary:") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeAcceptsShortFlags(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"-c", configPath,
		"-f", "console",
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "No matches found.") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeNoColorEnvOverridesConfigEnabledColors(t *testing.T) {
	setAnalyzeCwd(t)
	t.Setenv("NO_COLOR", "1")

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, configWithConsoleColorsEnabled(true))

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected NO_COLOR to disable ANSI output, got: %q", output.String())
	}
	if !strings.Contains(output.String(), "- ERROR 1:1 Found token token=abc") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeConfigEnabledColorsWithoutNoColorEnv(t *testing.T) {
	setAnalyzeCwd(t)
	t.Setenv("NO_COLOR", "")

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, configWithConsoleColorsEnabled(true))

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "\x1b[31mERROR\x1b[0m") {
		t.Fatalf("expected ANSI-colored error label, got: %q", output.String())
	}
}

func TestHandleAnalyzeConfigDisabledColorsWithoutNoColorEnv(t *testing.T) {
	setAnalyzeCwd(t)
	t.Setenv("NO_COLOR", "")

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, configWithConsoleColorsEnabled(false))

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		rootDir,
	}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("expected config to disable ANSI output, got: %q", output.String())
	}
	if !strings.Contains(output.String(), "- ERROR 1:1 Found token token=abc") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeReturnsErrorWhenFormatsInvalid(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--format", "bogus",
	}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "invalid format") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeRejectsGitModeDiffWithoutGitDiff(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--git-mode", "diff",
	}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "effective --git-mode=diff requires --git-diff") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestHandleAnalyzeRejectsGitAddedLinesOnlyWithGitModeOff(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	configPath := writeConfig(t, sampleConfig())

	var output bytes.Buffer
	code := HandleAnalyze([]string{
		"--config", configPath,
		"--git-added-lines-only",
	}, &output)

	if code != exitCodeError {
		t.Fatalf("expected exit code %d, got %d", exitCodeError, code)
	}
	if !strings.Contains(output.String(), "--git-added-lines-only is valid only when --git-mode=staged|diff") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestRunAnalyzeChecksGitCapabilitiesWhenModeEnabled(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	called := false
	var gotRequest git.CapabilityRequest
	restore := setCheckGitCapabilitiesHook(t, func(request git.CapabilityRequest) error {
		called = true
		gotRequest = request

		return errors.New("git mode staged requires git executable")
	})
	t.Cleanup(restore)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	})
	_ = result
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("expected git capability check to be invoked")
	}
	if gotRequest.Mode != "staged" {
		t.Fatalf("expected mode staged, got %q", gotRequest.Mode)
	}
	if gotRequest.WorkingDir != rootDir {
		t.Fatalf("expected working dir %q, got %q", rootDir, gotRequest.WorkingDir)
	}
	if !strings.Contains(err.Error(), "git mode staged requires git executable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAnalyzeSkipsGitCapabilitiesWhenModeOff(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	calls := 0
	restore := setCheckGitCapabilitiesHook(t, func(git.CapabilityRequest) error {
		calls++

		return errors.New("should not be called")
	})
	t.Cleanup(restore)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{"--config", configPath, rootDir})
	_ = result
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected 0 capability checks, got %d", calls)
	}
}

func TestRunAnalyzeRunsGitSelectionHooksWhenModeEnabled(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	restoreCapabilities := setCheckGitCapabilitiesHook(t, func(git.CapabilityRequest) error {
		return nil
	})
	t.Cleanup(restoreCapabilities)

	candidateCalls := 0
	addedLineCalls := 0
	restoreCandidates := setSelectGitCandidateFilesHook(t, func(request git.CandidateSelectionRequest) ([]string, error) {
		candidateCalls++
		if request.Mode != "staged" {
			t.Fatalf("expected mode staged, got %q", request.Mode)
		}
		if request.WorkingDir != rootDir {
			t.Fatalf("expected working dir %q, got %q", rootDir, request.WorkingDir)
		}

		return []string{"sample.txt"}, nil
	})
	t.Cleanup(restoreCandidates)
	restoreAddedLines := setSelectGitAddedLinesHook(
		t,
		func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			addedLineCalls++

			return nil, nil
		},
	)
	t.Cleanup(restoreAddedLines)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	})
	_ = result
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if candidateCalls != 1 {
		t.Fatalf("expected one candidate selection call, got %d", candidateCalls)
	}
	if addedLineCalls != 0 {
		t.Fatalf("expected zero added-line calls, got %d", addedLineCalls)
	}
}

func TestRunAnalyzeSkipsGitSelectionHooksWhenModeOff(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	restoreCandidates := setSelectGitCandidateFilesHook(t, func(git.CandidateSelectionRequest) ([]string, error) {
		t.Fatal("expected candidate selection hook not to run")

		return nil, nil
	})
	t.Cleanup(restoreCandidates)
	restoreAddedLines := setSelectGitAddedLinesHook(
		t,
		func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			t.Fatal("expected added-lines hook not to run")

			return nil, nil
		},
	)
	t.Cleanup(restoreAddedLines)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{"--config", configPath, rootDir})
	_ = result
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRunAnalyzeReturnsNoMatchesWhenGitCandidatesAreEmpty(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=abc")
	configPath := writeConfig(t, sampleConfig())

	restoreCapabilities := setCheckGitCapabilitiesHook(t, func(git.CapabilityRequest) error {
		return nil
	})
	t.Cleanup(restoreCapabilities)

	restoreCandidates := setSelectGitCandidateFilesHook(t, func(git.CandidateSelectionRequest) ([]string, error) {
		return []string{}, nil
	})
	t.Cleanup(restoreCandidates)

	restoreAddedLines := setSelectGitAddedLinesHook(
		t,
		func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			return nil, nil
		},
	)
	t.Cleanup(restoreAddedLines)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	})
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("expected no matches for empty candidate scope, got %d", len(result.Matches))
	}
	if result.Stats.FilesScanned != 0 {
		t.Fatalf("expected 0 scanned files for empty candidate scope, got %d", result.Stats.FilesScanned)
	}
}

func TestRunAnalyzeReturnsSelectionHookErrorWhenGitEnabled(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "clean")
	configPath := writeConfig(t, sampleConfig())

	restoreCapabilities := setCheckGitCapabilitiesHook(t, func(git.CapabilityRequest) error {
		return nil
	})
	t.Cleanup(restoreCapabilities)

	restoreCandidates := setSelectGitCandidateFilesHook(t, func(git.CandidateSelectionRequest) ([]string, error) {
		return nil, errors.New("candidate hook failed")
	})
	t.Cleanup(restoreCandidates)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{
		"--config", configPath,
		"--git-mode", "staged",
		rootDir,
	})
	_ = result
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "candidate hook failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAnalyzeFiltersMatchesByAddedLinesHook(t *testing.T) {
	t.Parallel()
	setAnalyzeCwd(t)

	rootDir := t.TempDir()
	writeFile(t, rootDir, "sample.txt", "token=aaa\ntoken=bbb\n")
	configPath := writeConfig(t, sampleConfig())

	restoreCapabilities := setCheckGitCapabilitiesHook(t, func(git.CapabilityRequest) error {
		return nil
	})
	t.Cleanup(restoreCapabilities)

	restoreCandidates := setSelectGitCandidateFilesHook(t, func(git.CandidateSelectionRequest) ([]string, error) {
		return []string{"sample.txt"}, nil
	})
	t.Cleanup(restoreCandidates)
	restoreAddedLines := setSelectGitAddedLinesHook(
		t,
		func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			return map[string]map[int]struct{}{"sample.txt": {2: {}}}, nil
		},
	)
	t.Cleanup(restoreAddedLines)

	result, failOn, formats, ruleSet, cfg, colors, err := runAnalyze([]string{
		"--config", configPath,
		"--git-mode", "staged",
		"--git-added-lines-only",
		rootDir,
	})
	_ = failOn
	_ = formats
	_ = ruleSet
	_ = cfg
	_ = colors
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected one match after added-lines filtering, got %d", len(result.Matches))
	}
	if result.Matches[0].Line != 2 {
		t.Fatalf("expected remaining match on line 2, got %d", result.Matches[0].Line)
	}
}

func TestSeverityRankUnknown(t *testing.T) {
	t.Parallel()

	if severityRank("bogus") != severityRankUnknown {
		t.Fatalf("unexpected severity rank")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(path, []byte(contents), defaultFileMode); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return path
}

func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), defaultFileMode); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func writeBaseline(t *testing.T, document baseline.Document) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "baseline.json")
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("failed to marshal baseline: %v", err)
	}
	if err := os.WriteFile(path, payload, defaultFileMode); err != nil {
		t.Fatalf("failed to write baseline: %v", err)
	}

	return path
}

func readBaseline(t *testing.T, path string) baseline.Document {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}

	var document baseline.Document
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}

	return document
}

func sampleConfig() string {
	return "rules:\n  - message: \"Found token $0\"\n    regex: \"token=[a-z]+\"\n    severity: \"error\"\n"
}

func configWithConsoleColorsEnabled(enabled bool) string {
	return "consoleColorsEnabled: " + fmt.Sprintf("%t", enabled) + "\n" + sampleConfig()
}

func setAnalyzeCwd(t *testing.T) {
	t.Helper()

	cwdMutex.Lock()
	t.Cleanup(func() {
		cwdMutex.Unlock()
	})

	currentRegistry := outputRegistry
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to read cwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("failed to change cwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(current)
		outputRegistry = currentRegistry
	})
}

func setCheckGitCapabilitiesHook(t *testing.T, hook func(git.CapabilityRequest) error) func() {
	t.Helper()

	original := checkGitCapabilities
	checkGitCapabilities = hook

	return func() {
		checkGitCapabilities = original
	}
}

func setSelectGitCandidateFilesHook(
	t *testing.T,
	hook func(git.CandidateSelectionRequest) ([]string, error),
) func() {
	t.Helper()

	original := selectGitCandidateFiles
	selectGitCandidateFiles = hook

	return func() {
		selectGitCandidateFiles = original
	}
}

func setSelectGitAddedLinesHook(
	t *testing.T,
	hook func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error),
) func() {
	t.Helper()

	original := selectGitAddedLines
	selectGitAddedLines = hook

	return func() {
		selectGitAddedLines = original
	}
}
