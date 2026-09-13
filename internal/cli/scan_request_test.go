package cli_test

import (
	"testing"

	"github.com/reglint/reglint/internal/cli"
	"github.com/reglint/reglint/internal/config"
	"github.com/reglint/reglint/internal/rules"
)

func TestBuildScanRequestOverridesIncludeExcludeAndFailOn(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Include:            []string{"src/**"},
		Exclude:            []string{"vendor/**"},
		FailOn:             stringPtr("warning"),
		IgnoreFilesEnabled: boolPtr(true),
		Rules: []config.Rule{
			{
				Message: "hello",
				Regex:   "world",
			},
		},
	}
	cfg := cli.Config{
		Roots:            []string{"."},
		Include:          []string{"**/*.go"},
		Exclude:          []string{"**/generated/**"},
		FailOnSeverity:   "error",
		NoIgnoreFiles:    true,
		Concurrency:      3,
		MaxFileSizeBytes: 99,
	}

	request, failOn, _ := cli.BuildScanRequest(cfg, ruleSet)

	assertEqualString(t, "fail-on severity", failOn, "error")
	assertStringSlice(t, "include override", request.Include, []string{"**/*.go"})
	assertStringSlice(t, "exclude override", request.Exclude, []string{"**/generated/**"})
	assertRuleCount(t, request.Rules, 1)
	assertStringSlice(t, "rule paths override", request.Rules[0].Paths, []string{"**/*.go"})
	assertStringSlice(t, "rule exclude override", request.Rules[0].Exclude, []string{"**/generated/**"})
	assertEqualString(t, "ruleset include unchanged", ruleSet.Include[0], "src/**")
	assertEqualString(t, "ruleset exclude unchanged", ruleSet.Exclude[0], "vendor/**")
}

func TestBuildScanRequestUsesRuleSetDefaultsWithoutOverrides(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      2,
		MaxFileSizeBytes: 10,
	}

	request, failOn, _ := cli.BuildScanRequest(cfg, ruleSet)

	expectedExclude := []string{"**/.git/**", "**/node_modules/**", "**/vendor/**"}
	assertEqualString(t, "fail-on severity", failOn, "")
	assertStringSlice(t, "default include", request.Include, []string{"**/*"})
	assertStringSlice(t, "default exclude", request.Exclude, expectedExclude)
	assertRuleCount(t, request.Rules, 1)
	assertStringSlice(t, "default rule paths", request.Rules[0].Paths, []string{"**/*"})
	assertStringSlice(t, "default rule exclude", request.Rules[0].Exclude, expectedExclude)
}

func TestBuildScanRequestUsesRuleSetConcurrencyWhenNotSetInCLI(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Concurrency: intPtr(7),
		Rules:       []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      2,
		MaxFileSizeBytes: 10,
		ConcurrencySet:   false,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Concurrency != 7 {
		t.Fatalf("expected concurrency 7, got %d", request.Concurrency)
	}
}

func TestBuildScanRequestUsesCLIConcurrencyWhenSet(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Concurrency: intPtr(7),
		Rules:       []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      3,
		MaxFileSizeBytes: 10,
		ConcurrencySet:   true,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Concurrency != 3 {
		t.Fatalf("expected concurrency 3, got %d", request.Concurrency)
	}
}

func TestBuildScanRequestResolvesIgnoreSettings(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		IgnoreFilesEnabled: boolPtr(false),
		IgnoreFiles:        []string{".customignore"},
		Rules:              []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
		NoIgnoreFiles:    true,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Ignore.Enabled {
		t.Fatal("expected ignore files to be disabled")
	}
	assertStringSlice(t, "ignore files", request.Ignore.Files, []string{".customignore"})
}

func TestBuildScanRequestUsesDefaultConsoleColorsWhenUnset(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	_, _, colorSettings := cli.BuildScanRequest(cfg, ruleSet)

	if !colorSettings.Enabled {
		t.Fatal("expected default console colors to be enabled")
	}
	if colorSettings.Source != "default" {
		t.Fatalf("expected default color source, got %q", colorSettings.Source)
	}
}

func TestBuildScanRequestUsesRuleSetConsoleColorsSetting(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	consoleColorsEnabled := false
	ruleSet := config.RuleSet{
		ConsoleColorsEnabled: &consoleColorsEnabled,
		Rules:                []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	_, _, colorSettings := cli.BuildScanRequest(cfg, ruleSet)

	if colorSettings.Enabled {
		t.Fatal("expected console colors to be disabled by ruleset")
	}
	if colorSettings.Source != "config" {
		t.Fatalf("expected config color source, got %q", colorSettings.Source)
	}
}

func TestBuildScanRequestNoColorEnvOverridesRuleSetConsoleColors(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	consoleColorsEnabled := true
	ruleSet := config.RuleSet{
		ConsoleColorsEnabled: &consoleColorsEnabled,
		Rules:                []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	_, _, colorSettings := cli.BuildScanRequest(cfg, ruleSet)

	if colorSettings.Enabled {
		t.Fatal("expected console colors to be disabled by NO_COLOR")
	}
	if colorSettings.Source != "env" {
		t.Fatalf("expected env color source, got %q", colorSettings.Source)
	}
}

func TestBuildScanRequestEmptyNoColorDoesNotOverrideRuleSet(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	consoleColorsEnabled := false
	ruleSet := config.RuleSet{
		ConsoleColorsEnabled: &consoleColorsEnabled,
		Rules:                []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	_, _, colorSettings := cli.BuildScanRequest(cfg, ruleSet)

	if colorSettings.Enabled {
		t.Fatal("expected console colors to stay disabled by ruleset")
	}
	if colorSettings.Source != "config" {
		t.Fatalf("expected config color source, got %q", colorSettings.Source)
	}
}

func TestBuildScanRequestUsesRuleSetGitSettingsWithoutCLIOverrides(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Git: &config.GitSettings{
			Mode:             stringPtr("staged"),
			AddedLinesOnly:   boolPtr(true),
			GitignoreEnabled: boolPtr(true),
		},
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Git == nil {
		t.Fatal("expected git request to be set")
	}
	if request.Git.Mode != "staged" {
		t.Fatalf("expected git mode staged, got %q", request.Git.Mode)
	}
	if !request.Git.AddedLinesOnly {
		t.Fatal("expected added-lines-only true")
	}
	if !request.Git.GitignoreEnabled {
		t.Fatal("expected gitignore enabled")
	}
}

func TestBuildScanRequestCLIOverridesRuleSetGitSettings(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Git: &config.GitSettings{
			Mode:             stringPtr("off"),
			Diff:             stringPtr("HEAD~2..HEAD~1"),
			AddedLinesOnly:   boolPtr(false),
			GitignoreEnabled: boolPtr(true),
		},
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:                []string{"./root"},
		Concurrency:          1,
		MaxFileSizeBytes:     10,
		GitMode:              "staged",
		GitModeSet:           true,
		GitAddedLinesOnly:    true,
		GitAddedLinesOnlySet: true,
		NoGitignore:          true,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Git == nil {
		t.Fatal("expected git request to be set")
	}
	if request.Git.Mode != "staged" {
		t.Fatalf("expected git mode staged, got %q", request.Git.Mode)
	}
	if request.Git.DiffTarget != "" {
		t.Fatalf("expected empty diff target for non-diff mode, got %q", request.Git.DiffTarget)
	}
	if !request.Git.AddedLinesOnly {
		t.Fatal("expected added-lines-only true")
	}
	if request.Git.GitignoreEnabled {
		t.Fatal("expected gitignore disabled by CLI")
	}
}

func TestBuildScanRequestGitDiffForcesDiffMode(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Git:   &config.GitSettings{Mode: stringPtr("staged")},
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
		GitMode:          "off",
		GitModeSet:       true,
		GitDiffTarget:    "HEAD~1..HEAD",
		GitDiffSet:       true,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Git == nil {
		t.Fatal("expected git request to be set")
	}
	if request.Git.Mode != "diff" {
		t.Fatalf("expected diff mode, got %q", request.Git.Mode)
	}
	if request.Git.DiffTarget != "HEAD~1..HEAD" {
		t.Fatalf("expected diff target HEAD~1..HEAD, got %q", request.Git.DiffTarget)
	}
}

func TestBuildScanRequestGitModeOffReturnsNilGitRequest(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Git:   &config.GitSettings{Mode: stringPtr("off")},
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Git != nil {
		t.Fatal("expected nil git request in off mode")
	}
}

func TestBuildScanRequestGitModeOffIncludesGitignoreByDefault(t *testing.T) {
	t.Parallel()

	ruleSet := config.RuleSet{
		Rules: []config.Rule{{Message: "hello", Regex: "world"}},
	}
	cfg := cli.Config{
		Roots:            []string{"./root"},
		Concurrency:      1,
		MaxFileSizeBytes: 10,
	}

	request, _, _ := cli.BuildScanRequest(cfg, ruleSet)

	if request.Git != nil {
		t.Fatal("expected nil git request in off mode")
	}
	assertStringSlice(t, "ignore files", request.Ignore.Files, []string{".gitignore", ".ignore", ".reglintignore"})
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func assertRuleCount(t *testing.T, rules []rules.Rule, expected int) {
	t.Helper()

	if len(rules) != expected {
		t.Fatalf("expected %d rule(s), got %d", expected, len(rules))
	}
}

func assertStringSlice(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %s %v, got %v", label, want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %s %v, got %v", label, want, got)
		}
	}
}

func assertEqualString(t *testing.T, label, got, want string) {
	t.Helper()

	if got != want {
		t.Fatalf("expected %s %q, got %q", label, want, got)
	}
}
