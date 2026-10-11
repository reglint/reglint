package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestE2EHarnessRunExecutesCompiledBinary(t *testing.T) {
	harness := newE2EHarness(t)

	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "rules.yaml")
	scenario := e2EScenario{
		ID:           "E2E-HARNESS-001",
		Tier:         "smoke",
		Name:         "init command creates a config file",
		Fixture:      workspace,
		Command:      []string{"init", "--out", configPath},
		ExpectedExit: 0,
	}

	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStdoutContains(t, scenario, result, "Wrote default config to "+configPath)
	harness.assertScenarioStderrEmpty(t, scenario, result)
	harness.assertScenarioPathExists(t, scenario, result, configPath)
}

func TestE2EHarnessBuildsBinaryOnlyOnce(t *testing.T) {
	harness1 := newE2EHarness(t)
	workspace1 := t.TempDir()
	path1 := filepath.Join(workspace1, "rules.yaml")
	scenario1 := e2EScenario{
		ID:           "E2E-HARNESS-002",
		Tier:         "smoke",
		Name:         "first run uses built binary",
		Fixture:      workspace1,
		Command:      []string{"init", "--out", path1},
		ExpectedExit: 0,
	}
	harness1.mustRunScenario(t, scenario1)

	buildCountAfterFirst := e2eBinaryBuildInvocations()

	harness2 := newE2EHarness(t)
	workspace2 := t.TempDir()
	path2 := filepath.Join(workspace2, "rules.yaml")
	scenario2 := e2EScenario{
		ID:           "E2E-HARNESS-003",
		Tier:         "smoke",
		Name:         "second run reuses built binary",
		Fixture:      workspace2,
		Command:      []string{"init", "--out", path2},
		ExpectedExit: 0,
	}
	harness2.mustRunScenario(t, scenario2)

	buildCountAfterSecond := e2eBinaryBuildInvocations()
	if buildCountAfterSecond != buildCountAfterFirst {
		t.Fatalf("expected binary build count to stay at %d, got %d", buildCountAfterFirst, buildCountAfterSecond)
	}
}

func TestE2EHarnessFailureDiagnosticsIncludeScenarioMetadata(t *testing.T) {
	t.Parallel()

	harness := &e2EHarness{binaryPath: filepath.Join(t.TempDir(), "reglint binary")}
	scenario := e2EScenario{
		ID:           "E2E-SMOKE-999",
		Tier:         "smoke",
		Name:         "metadata diagnostics",
		Fixture:      filepath.Join(t.TempDir(), "fixture workspace"),
		Command:      []string{"analyze", "--config", "rules.yaml", "./fixtures"},
		Env:          map[string]string{"NO_COLOR": "1", "TEST_MODE": "e2e"},
		ExpectedExit: 2,
	}
	result := e2EProcessResult{ExitCode: 1, Stdout: "stdout-body", Stderr: "stderr-body"}

	diagnostic := harness.scenarioFailureDiagnostic(scenario, result, "expected exit code 2, got 1")

	if !strings.Contains(diagnostic, "scenario: E2E-SMOKE-999") {
		t.Fatalf("expected scenario id in diagnostic, got %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "fixture: "+scenario.Fixture) {
		t.Fatalf("expected fixture path in diagnostic, got %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "replay: ") {
		t.Fatalf("expected replay command in diagnostic, got %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "NO_COLOR=") || !strings.Contains(diagnostic, "TEST_MODE=") {
		t.Fatalf("expected env overrides in replay command, got %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "stdout: \"stdout-body\"") {
		t.Fatalf("expected stdout capture in diagnostic, got %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "stderr: \"stderr-body\"") {
		t.Fatalf("expected stderr capture in diagnostic, got %q", diagnostic)
	}
}

func TestE2EHarnessAssertionCatalogPassesDeterministicChecks(t *testing.T) {
	t.Parallel()

	harness := &e2EHarness{}
	workspace := t.TempDir()

	jsonPath := filepath.Join(workspace, "result.json")
	if err := os.WriteFile(jsonPath, []byte(`{"stats":{"matches":2},"schemaVersion":1}`), 0o600); err != nil {
		t.Fatalf("write json fixture: %v", err)
	}

	sarifPath := filepath.Join(workspace, "result.sarif")
	sarifPayload := []byte(
		`{"runs":[{"results":[{"level":"warning"}]}],"version":"2.1.0"}`,
	)
	if err := os.WriteFile(sarifPath, sarifPayload, 0o600); err != nil {
		t.Fatalf("write sarif fixture: %v", err)
	}

	scenario := e2EScenario{
		ID:      "E2E-HARNESS-ASSERT-001",
		Tier:    "smoke",
		Name:    "assertion catalog",
		Fixture: workspace,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "summary ready"},
			{Type: e2EAssertionStderrContains, Value: "warning-42"},
			{Type: e2EAssertionStdoutNotContains, Value: "secret="},
			{Type: e2EAssertionStderrNotContains, Value: "panic"},
			{Type: e2EAssertionStdoutRegex, Value: `summary\s+ready`},
			{Type: e2EAssertionStderrRegex, Value: `warning-\d+`},
			{Type: e2EAssertionFileExists, Path: jsonPath},
			{Type: e2EAssertionFileNotExists, Path: filepath.Join(workspace, "missing.txt")},
			{Type: e2EAssertionJSONFieldEquals, Path: jsonPath, Field: "stats.matches", Expected: 2},
			{Type: e2EAssertionSARIFFieldEquals, Path: sarifPath, Field: "runs.0.results.0.level", Expected: "warning"},
		},
	}

	result := e2EProcessResult{
		Stdout: "summary ready",
		Stderr: "warning-42",
	}

	harness.assertScenarioAssertions(t, scenario, result)
}

func TestE2EHarnessJSONFieldAssertionReadsStdoutWhenPathUnset(t *testing.T) {
	t.Parallel()

	harness := &e2EHarness{}
	scenario := e2EScenario{ID: "E2E-HARNESS-ASSERT-002", Tier: "smoke", Name: "json from stdout"}
	result := e2EProcessResult{Stdout: `{"stats":{"matches":1}}`}

	err := harness.evaluateScenarioAssertion(scenario, result, e2EAssertion{
		Type:     e2EAssertionJSONFieldEquals,
		Field:    "stats.matches",
		Expected: 1,
	})
	if err != nil {
		t.Fatalf("expected json assertion from stdout to pass, got %v", err)
	}
}

func TestE2EHarnessAssertionRejectsUnknownType(t *testing.T) {
	t.Parallel()

	harness := &e2EHarness{}
	scenario := e2EScenario{ID: "E2E-HARNESS-ASSERT-003", Tier: "smoke", Name: "unknown assertion type"}

	err := harness.evaluateScenarioAssertion(scenario, e2EProcessResult{}, e2EAssertion{Type: "unsupported"})
	if err == nil {
		t.Fatal("expected unknown assertion type to fail")
	}
}

func TestE2EHarnessScenarioOrderingByTierThenID(t *testing.T) {
	t.Parallel()

	input := []e2EScenario{
		{ID: "E2E-FULL-002", Tier: "full", Name: "full 2"},
		{ID: "E2E-SMOKE-003", Tier: "smoke", Name: "smoke 3"},
		{ID: "E2E-SMOKE-001", Tier: "smoke", Name: "smoke 1"},
		{ID: "E2E-FULL-001", Tier: "full", Name: "full 1"},
	}

	got := sortE2EScenarios(input)

	if len(got) != 4 {
		t.Fatalf("expected 4 scenarios, got %d", len(got))
	}

	wantOrder := []string{"E2E-SMOKE-001", "E2E-SMOKE-003", "E2E-FULL-001", "E2E-FULL-002"}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Fatalf("unexpected scenario ordering at index %d: want %s, got %s", i, want, got[i].ID)
		}
	}

	if input[0].ID != "E2E-FULL-002" {
		t.Fatalf("expected sort helper to avoid mutating caller slice, got first input id %s", input[0].ID)
	}
}

func TestE2ESmoke001AnalyzeHappyPathDeterministicSummary(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke001Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2ESmoke002InvalidConfigSingleActionableError(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke002Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2ESmoke003FailOnThresholdExceeded(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke003Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2ESmoke004NoFindingsExitZero(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke004Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2ESmoke005NoColorDisablesANSIOutput(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke005Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2ESmoke006PathWithSpacesCorrectPathReporting(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2ESmoke006Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull001BaselineCompareSuppressesNonRegressions(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2EFull001Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull002BaselineWriteOverwritesTargetAndExitsZero(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("seed baseline file: %v", err)
	}

	scenario := newE2EFull002Scenario(moduleRoot, baselinePath)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull003BaselinePathPrecedenceCLIOverridesRuleSet(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	baselinePath := filepath.Join(t.TempDir(), "cli-baseline.json")
	if err := os.WriteFile(baselinePath, []byte(`{"schemaVersion":1,"entries":[]}`), 0o600); err != nil {
		t.Fatalf("seed cli baseline file: %v", err)
	}

	scenario := newE2EFull003Scenario(moduleRoot, baselinePath)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull004JSONOnlyFormatWritesToStdoutWhenOutPathUnset(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2EFull004Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull005SARIFOnlyFormatWritesToStdoutWhenOutPathUnset(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	scenario := newE2EFull005Scenario(moduleRoot)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull006MultiFormatRequiresExplicitOutputPaths(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	outJSONPath := filepath.Join(t.TempDir(), "result.json")
	scenario := newE2EFull006Scenario(moduleRoot, outJSONPath)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull007GitModeOffWorksWhenGitExecutableUnavailable(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, ".gitignore", "sample.txt\n")
	writeFixture(t, fixtureDir, "sample.txt", "token=abc\n")

	scenario := newE2EFull007Scenario(moduleRoot, fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull008GitModeStagedScansOnlyStagedFiles(t *testing.T) {
	ensureGitAvailable(t)

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "staged.txt", "token=aaa\n")
	writeFixture(t, repoDir, "unstaged.txt", "token=bbb\n")
	runGit(t, repoDir, "add", "staged.txt")

	scenario := newE2EFull008Scenario(moduleRoot, repoDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull009GitModeDiffScansOnlyDiffSelectedFiles(t *testing.T) {
	ensureGitAvailable(t)

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "changed.txt", "clean\n")
	writeFixture(t, repoDir, "unchanged.txt", "token=abc\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	writeFixture(t, repoDir, "changed.txt", "token=xyz\n")

	scenario := newE2EFull009Scenario(moduleRoot, repoDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull010GitAddedLinesOnlyReportsOnlyMatchesOnAddedLines(t *testing.T) {
	ensureGitAvailable(t)

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "sample.txt", "token=old\nclean\n")
	runGit(t, repoDir, "add", "sample.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	writeFixture(t, repoDir, "sample.txt", "token=old\nclean\ntoken=new\n")
	runGit(t, repoDir, "add", "sample.txt")

	scenario := newE2EFull010Scenario(moduleRoot, repoDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull011GitEnabledRunOutsideRepoExitsWithSingleError(t *testing.T) {
	ensureGitAvailable(t)

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	nonRepoDir := t.TempDir()
	writeFixture(t, nonRepoDir, "sample.txt", "token=abc\n")

	scenario := newE2EFull011Scenario(moduleRoot, nonRepoDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull012BinaryAndOversizedFilesSkippedWithDeterministicStats(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "scannable.txt", "token=abc\n")

	binaryPath := filepath.Join(fixtureDir, "binary.bin")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	oversizedPath := filepath.Join(fixtureDir, "oversized.txt")
	if err := os.WriteFile(oversizedPath, []byte("token=abcdefghij\n"), 0o600); err != nil {
		t.Fatalf("write oversized fixture: %v", err)
	}

	scenario := newE2EFull012Scenario(moduleRoot, fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull013UnreadableFilesRecordErrorsWhileScanContinues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions are unreliable on Windows")
	}

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "scannable.txt", "token=abc\n")
	blockedPath := writeFixture(t, fixtureDir, "unreadable.txt", "token=blocked\n")

	if err := os.Chmod(blockedPath, 0o000); err != nil {
		t.Fatalf("set unreadable permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blockedPath, 0o600)
	})

	scenario := newE2EFull013Scenario(moduleRoot, fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull014IgnorePrecedenceDeterministicReglintignoreOverrides(t *testing.T) {
	ensureGitAvailable(t)

	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeFixture(t, repoDir, "ignore-wins.txt", "token=ignorewins\n")
	writeFixture(t, repoDir, "reglintignore-wins.txt", "token=reglintignorewins\n")
	writeFixture(t, repoDir, ".gitignore", "ignore-wins.txt\nreglintignore-wins.txt\n")
	writeFixture(t, repoDir, ".ignore", "!ignore-wins.txt\n!reglintignore-wins.txt\n")
	writeFixture(t, repoDir, ".reglintignore", "reglintignore-wins.txt\n")
	runGit(t, repoDir, "add", "-f", "ignore-wins.txt", "reglintignore-wins.txt")

	scenario := newE2EFull014Scenario(moduleRoot, repoDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull015RepeatedRunsProduceStableOrdering(t *testing.T) {
	harness := newE2EHarness(t)

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "b.txt", "token=bbb\n")
	writeFixture(t, fixtureDir, "a.txt", "token=aaa\ntoken=ccc\n")

	scenario := newE2EFull015Scenario(moduleRoot, fixtureDir)
	first := harness.mustRunScenario(t, scenario)
	second := harness.mustRunScenario(t, scenario)
	third := harness.mustRunScenario(t, scenario)

	harness.assertScenarioStderrEmpty(t, scenario, first)
	harness.assertScenarioStderrEmpty(t, scenario, second)
	harness.assertScenarioStderrEmpty(t, scenario, third)

	firstOrder := extractMatchOrderFromJSON(t, first.Stdout)
	secondOrder := extractMatchOrderFromJSON(t, second.Stdout)
	thirdOrder := extractMatchOrderFromJSON(t, third.Stdout)

	if !reflect.DeepEqual(firstOrder, secondOrder) {
		t.Fatalf("expected second run ordering %v to equal first run %v", secondOrder, firstOrder)
	}
	if !reflect.DeepEqual(firstOrder, thirdOrder) {
		t.Fatalf("expected third run ordering %v to equal first run %v", thirdOrder, firstOrder)
	}
}

func TestE2EFull016ConfigRuleFilesLoadRulesFromSeparateFiles(t *testing.T) {
	harness := newE2EHarness(t)

	fixtureDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureDir, "src"), 0o755); err != nil {
		t.Fatalf("create src dir: %v", err)
	}
	writeFixture(t, fixtureDir, filepath.Join("src", "sample.txt"), "token=abc\n")
	writeFixture(t, fixtureDir, "reglint.yaml", "ruleFiles:\n  - pack.yaml\n")
	writeFixture(t, fixtureDir, "pack.yaml",
		"rules:\n  - message: \"Found token\"\n    regex: \"token=\"\n    severity: \"warning\"\n")

	scenario := newE2EFull016Scenario(fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull017RulesFlagLoadsRuleFilesAfterConfigRules(t *testing.T) {
	harness := newE2EHarness(t)

	fixtureDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureDir, "src"), 0o755); err != nil {
		t.Fatalf("create src dir: %v", err)
	}
	writeFixture(t, fixtureDir, filepath.Join("src", "sample.txt"), "alpha=1\nbeta=2\n")
	writeFixture(t, fixtureDir, "reglint.yaml", "rules:\n  - message: \"Alpha\"\n    regex: \"alpha=\"\n")
	writeFixture(t, fixtureDir, "pack.yaml", "rules:\n  - message: \"Beta\"\n    regex: \"beta=\"\n")

	scenario := newE2EFull017Scenario(fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func TestE2EFull018RuleFileWithSettingsExitsWithSingleError(t *testing.T) {
	harness := newE2EHarness(t)

	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "sample.txt", "token=abc\n")
	writeFixture(t, fixtureDir, "reglint.yaml", "rules:\n  - message: \"Found token\"\n    regex: \"token=\"\n")
	writeFixture(t, fixtureDir, "bad.yaml", "rules:\n  - message: \"X\"\n    regex: \"x\"\nfailOn: error\n")

	scenario := newE2EFull018Scenario(fixtureDir)
	result := harness.mustRunScenario(t, scenario)
	harness.assertScenarioStderrEmpty(t, scenario, result)
}

func extractMatchOrderFromJSON(t *testing.T, payload string) []string {
	t.Helper()

	var document struct {
		Matches []struct {
			FilePath string `json:"filePath"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(payload), &document); err != nil {
		t.Fatalf("decode json output: %v", err)
	}

	order := make([]string, 0, len(document.Matches))
	for _, match := range document.Matches {
		order = append(
			order,
			match.FilePath+":"+
				strconv.Itoa(match.Line)+":"+
				strconv.Itoa(match.Column)+":"+
				match.Severity+":"+
				match.Message,
		)
	}

	return order
}
