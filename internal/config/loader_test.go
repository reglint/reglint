package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/config"
)

func TestLoadRuleSetRejectsEmptyRules(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules: []\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rules must be") {
		t.Fatalf("expected rules error, got %v", err)
	}
}

func TestLoadRuleSetRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules: [")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsIncludeNotList(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "include: 'src/**'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsExcludeNotList(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "exclude: 'vendor/**'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsIncludeNonStringItems(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "include:\n  - 1\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsExcludeNonStringItems(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "exclude:\n  - true\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsRulePathsNotList(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    paths: 'src/**'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsRulePathsNonStringItems(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    paths:\n      - 1\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsRuleExcludeNotList(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    exclude: 'vendor/**'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsRuleExcludeNonStringItems(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    exclude:\n      - true\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetParsesConfig(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n")

	rules, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rules.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules.Rules))
	}
	if rules.Rules[0].Message != "hello" || rules.Rules[0].Regex != "world" {
		t.Fatalf("unexpected rule contents: %+v", rules.Rules[0])
	}
}

func TestLoadRuleSetAllowsFailOnAndConcurrency(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "failOn: 'warning'\nconcurrency: 2\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadRuleSetAllowsBaselinePath(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "baseline: 'testdata/baseline.json'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	ruleSet, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ruleSet.Baseline == nil {
		t.Fatal("expected baseline to be set")
	}
	if *ruleSet.Baseline != "testdata/baseline.json" {
		t.Fatalf("expected baseline path to be propagated, got %q", *ruleSet.Baseline)
	}
}

func TestLoadRuleSetParsesGitSettings(t *testing.T) {
	t.Parallel()

	gitConfig := "git:\n" +
		"  mode: 'diff'\n" +
		"  diff: 'HEAD~1..HEAD'\n" +
		"  addedLinesOnly: true\n" +
		"  gitignoreEnabled: false\n" +
		"rules:\n" +
		"  - message: 'hello'\n" +
		"    regex: 'world'\n"

	path := writeConfigFile(
		t,
		gitConfig,
	)

	ruleSet, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertGitSettings(t, ruleSet.Git)
}

func TestLoadRuleSetRejectsInvalidGitMode(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "git:\n  mode: 'invalid'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git.mode must be one of off, staged, diff") {
		t.Fatalf("expected git.mode validation error, got %v", err)
	}
}

func TestLoadRuleSetRejectsEmptyGitDiff(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "git:\n  mode: 'diff'\n  diff: ''\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git.diff must be a non-empty string") {
		t.Fatalf("expected git.diff validation error, got %v", err)
	}
}

func TestLoadRuleSetRejectsGitDiffOutsideDiffMode(t *testing.T) {
	t.Parallel()

	configContents := "git:\n" +
		"  mode: 'staged'\n" +
		"  diff: 'HEAD~1..HEAD'\n" +
		"rules:\n" +
		"  - message: 'hello'\n" +
		"    regex: 'world'\n"
	path := writeConfigFile(t, configContents)

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git.diff is valid only when git.mode=diff") {
		t.Fatalf("expected git.diff cross-field validation error, got %v", err)
	}
}

func TestLoadRuleSetRejectsGitModeDiffWithoutDiff(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "git:\n  mode: 'diff'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git.mode=diff requires git.diff") {
		t.Fatalf("expected git.mode diff requirement error, got %v", err)
	}
}

func TestLoadRuleSetRejectsGitAddedLinesOnlyWithoutGitMode(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "git:\n  addedLinesOnly: true\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git.addedLinesOnly=true is valid only when git.mode=staged|diff") {
		t.Fatalf("expected git.addedLinesOnly validation error, got %v", err)
	}
}

func TestLoadRuleSetAllowsGitAddedLinesOnlyWithStagedMode(t *testing.T) {
	t.Parallel()

	configContents := "git:\n" +
		"  mode: 'staged'\n" +
		"  addedLinesOnly: true\n" +
		"rules:\n" +
		"  - message: 'hello'\n" +
		"    regex: 'world'\n"
	path := writeConfigFile(t, configContents)

	_, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadRuleSetRejectsEmptyBaseline(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "baseline: ''\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("expected baseline validation error, got %v", err)
	}
}

func TestLoadRuleSetRejectsNonStringBaseline(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "baseline: 42\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "baseline must be a string" {
		t.Fatalf("expected baseline type validation error, got %v", err)
	}
}

func TestLoadRuleSetAllowsRuleSeverity(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    severity: 'info'\n")

	_, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadRuleSetRejectsRuleMissingMessage(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsRuleMissingRegex(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsInvalidFailOn(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "failOn: 'fatal'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsNonPositiveConcurrencyReportsValue(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "concurrency: 0\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "concurrency must be positive") {
		t.Fatalf("expected concurrency error, got %v", err)
	}
}

func TestLoadRuleSetRejectsIgnoreFilesWithEmptyValue(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "ignoreFiles:\n  - ''\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsIgnoreFilesWithSeparators(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "ignoreFiles:\n  - 'dir/.ignore'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsIgnoreFilesWithDuplicates(t *testing.T) {
	t.Parallel()

	configContents := "ignoreFiles:\n" +
		"  - '.ignore'\n" +
		"  - '.ignore'\n" +
		"rules:\n" +
		"  - message: 'hello'\n" +
		"    regex: 'world'\n"
	path := writeConfigFile(t, configContents)

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetAllowsIgnoreFilesEnabled(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "ignoreFilesEnabled: false\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadRuleSetParsesConsoleColorsEnabled(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "consoleColorsEnabled: false\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	rules, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rules.ConsoleColorsEnabled == nil {
		t.Fatal("expected consoleColorsEnabled to be set")
	}
	if *rules.ConsoleColorsEnabled {
		t.Fatal("expected consoleColorsEnabled to be false")
	}
}

func TestLoadRuleSetParsesConsoleColorsEnabledTrue(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "consoleColorsEnabled: true\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	rules, err := config.LoadRuleSet(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rules.ConsoleColorsEnabled == nil {
		t.Fatal("expected consoleColorsEnabled to be set")
	}
	if !*rules.ConsoleColorsEnabled {
		t.Fatal("expected consoleColorsEnabled to be true")
	}
}

func TestLoadRuleSetRejectsConsoleColorsEnabledNonBoolean(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "consoleColorsEnabled: 'false'\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsConsoleColorsEnabledNull(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "consoleColorsEnabled: null\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "consoleColorsEnabled must be a boolean") {
		t.Fatalf("expected consoleColorsEnabled validation error, got %v", err)
	}
}

func TestLoadRuleSetRejectsNonPositiveConcurrency(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "concurrency: 0\nrules:\n  - message: 'hello'\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsInvalidRuleSeverity(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: 'world'\n    severity: 'fatal'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsInvalidRegex(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: 'hello'\n    regex: '(['\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadRuleSetRejectsBlankRuleMessage(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "rules:\n  - message: '  '\n    regex: 'world'\n")

	_, err := config.LoadRuleSet(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRuleSetToRulesCopiesSlices(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Include: config.StringList{"src/**"},
		Exclude: config.StringList{"vendor/**"},
		Rules: []config.Rule{
			{
				Message:  "hello",
				Regex:    "world",
				Severity: "warning",
				Paths:    []string{"src/**"},
				Exclude:  []string{"src/vendor/**"},
			},
		},
	}

	converted := ruleSet.ToRules()
	converted.Include[0] = "changed/**"
	converted.Exclude[0] = "changed/**"
	converted.Rules[0].Paths[0] = "changed/**"
	converted.Rules[0].Exclude[0] = "changed/**"

	if ruleSet.Include[0] == "changed/**" || ruleSet.Exclude[0] == "changed/**" {
		t.Fatal("expected include/exclude to be copied")
	}
	if ruleSet.Rules[0].Paths[0] == "changed/**" || ruleSet.Rules[0].Exclude[0] == "changed/**" {
		t.Fatal("expected rule paths/exclude to be copied")
	}
}

func TestRuleSetToRulesDefaultsSeverity(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{
			{
				Message: "hello",
				Regex:   "world",
			},
		},
	}

	converted := ruleSet.ToRules()
	if converted.Rules[0].Severity != "warning" {
		t.Fatalf("expected default severity warning, got %q", converted.Rules[0].Severity)
	}
}

func TestRuleSetToRulesDefaultsRulePathsAndExclude(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{
			{
				Message: "hello",
				Regex:   "world",
			},
		},
	}

	converted := ruleSet.ToRules()
	if len(converted.Rules[0].Paths) != 1 || converted.Rules[0].Paths[0] != "**/*" {
		t.Fatalf("expected rule paths to default to ruleset include, got %v", converted.Rules[0].Paths)
	}
	if len(converted.Rules[0].Exclude) != 3 {
		t.Fatalf("expected rule exclude to default, got %v", converted.Rules[0].Exclude)
	}
}

func TestRuleSetToRulesDefaultsConcurrency(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Concurrency == nil {
		t.Fatal("expected concurrency default, got nil")
	}
}

func TestRuleSetToRulesPropagatesConsoleColorsEnabled(t *testing.T) {
	t.Parallel()

	consoleColorsEnabled := false
	ruleSet := config.RuleSet{
		ConsoleColorsEnabled: &consoleColorsEnabled,
		Rules:                []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.ConsoleColorsEnabled == nil {
		t.Fatal("expected consoleColorsEnabled to be propagated")
	}
	if *converted.ConsoleColorsEnabled {
		t.Fatal("expected propagated consoleColorsEnabled to be false")
	}
}

func TestRuleSetToRulesPropagatesBaseline(t *testing.T) {
	t.Parallel()

	baseline := "testdata/baseline.json"
	ruleSet := config.RuleSet{
		Baseline: &baseline,
		Rules:    []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Baseline == nil {
		t.Fatal("expected baseline to be propagated")
	}
	if *converted.Baseline != baseline {
		t.Fatalf("expected propagated baseline %q, got %q", baseline, *converted.Baseline)
	}
}

func TestRuleSetToRulesCopiesBaselineValue(t *testing.T) {
	t.Parallel()

	baseline := "testdata/baseline.json"
	ruleSet := config.RuleSet{
		Baseline: &baseline,
		Rules:    []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Baseline == nil {
		t.Fatal("expected baseline to be propagated")
	}

	*converted.Baseline = "changed.json"
	if *ruleSet.Baseline != "testdata/baseline.json" {
		t.Fatalf("expected original baseline to remain unchanged, got %q", *ruleSet.Baseline)
	}
}

func TestRuleSetToRulesLeavesBaselineUnsetWhenMissing(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Baseline != nil {
		t.Fatal("expected baseline to remain unset")
	}
}

func TestRuleSetToRulesDefaultsGitSettings(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Git.Mode != "off" {
		t.Fatalf("expected git mode off, got %q", converted.Git.Mode)
	}
	if converted.Git.Diff != "" {
		t.Fatalf("expected git diff to be empty by default, got %q", converted.Git.Diff)
	}
	if converted.Git.AddedLinesOnly {
		t.Fatal("expected git addedLinesOnly to default to false")
	}
	if !converted.Git.GitignoreEnabled {
		t.Fatal("expected git gitignoreEnabled to default to true")
	}
}

func TestRuleSetToRulesPropagatesGitSettings(t *testing.T) {
	t.Parallel()

	mode := "diff"
	diff := "HEAD~1..HEAD"
	addedLinesOnly := true
	gitignoreEnabled := false
	ruleSet := config.RuleSet{
		Git: &config.GitSettings{
			Mode:             &mode,
			Diff:             &diff,
			AddedLinesOnly:   &addedLinesOnly,
			GitignoreEnabled: &gitignoreEnabled,
		},
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.Git.Mode != "diff" {
		t.Fatalf("expected git mode diff, got %q", converted.Git.Mode)
	}
	if converted.Git.Diff != "HEAD~1..HEAD" {
		t.Fatalf("expected git diff HEAD~1..HEAD, got %q", converted.Git.Diff)
	}
	if !converted.Git.AddedLinesOnly {
		t.Fatal("expected git addedLinesOnly true")
	}
	if converted.Git.GitignoreEnabled {
		t.Fatal("expected git gitignoreEnabled false")
	}
}

func TestRuleSetToRulesLeavesConsoleColorsEnabledUnsetWhenMissing(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}

	converted := ruleSet.ToRules()
	if converted.ConsoleColorsEnabled != nil {
		t.Fatal("expected consoleColorsEnabled to remain unset")
	}
}

func assertGitSettings(t *testing.T, git *config.GitSettings) {
	t.Helper()

	if git == nil {
		t.Fatal("expected git settings to be set")
	}
	if git.Mode == nil || *git.Mode != "diff" {
		t.Fatalf("expected git.mode to be diff, got %+v", git.Mode)
	}
	if git.Diff == nil || *git.Diff != "HEAD~1..HEAD" {
		t.Fatalf("expected git.diff to be propagated, got %+v", git.Diff)
	}
	if git.AddedLinesOnly == nil || !*git.AddedLinesOnly {
		t.Fatalf("expected git.addedLinesOnly to be true, got %+v", git.AddedLinesOnly)
	}
	if git.GitignoreEnabled == nil || *git.GitignoreEnabled {
		t.Fatalf("expected git.gitignoreEnabled to be false, got %+v", git.GitignoreEnabled)
	}
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return path
}
