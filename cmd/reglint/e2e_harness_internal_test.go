package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type e2EHarness struct {
	binaryPath string
}

type e2EProcessResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type e2EScenario struct {
	ID           string
	Tier         string
	Name         string
	Fixture      string
	Command      []string
	Env          map[string]string
	ExpectedExit int
	Assertions   []e2EAssertion
}

type e2EAssertion struct {
	Type     string
	Value    string
	Path     string
	Field    string
	Expected any
}

type e2EAssertionEvaluator func(
	h *e2EHarness,
	scenario e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error

const (
	e2EAssertionStdoutContains    = "stdoutContains"
	e2EAssertionStderrContains    = "stderrContains"
	e2EAssertionStdoutNotContains = "stdoutNotContains"
	e2EAssertionStderrNotContains = "stderrNotContains"
	e2EAssertionStdoutRegex       = "stdoutRegex"
	e2EAssertionStderrRegex       = "stderrRegex"
	e2EAssertionFileExists        = "fileExists"
	e2EAssertionFileNotExists     = "fileNotExists"
	e2EAssertionJSONFieldEquals   = "jsonFieldEquals"
	e2EAssertionSARIFFieldEquals  = "sarifFieldEquals"
)

var e2EAssertionEvaluators = map[string]e2EAssertionEvaluator{
	e2EAssertionStdoutContains:    evalStdoutContains,
	e2EAssertionStderrContains:    evalStderrContains,
	e2EAssertionStdoutNotContains: evalStdoutNotContains,
	e2EAssertionStderrNotContains: evalStderrNotContains,
	e2EAssertionStdoutRegex:       evalStdoutRegex,
	e2EAssertionStderrRegex:       evalStderrRegex,
	e2EAssertionFileExists:        evalFileExists,
	e2EAssertionFileNotExists:     evalFileNotExists,
	e2EAssertionJSONFieldEquals: func(
		h *e2EHarness,
		scenario e2EScenario,
		result e2EProcessResult,
		assertion e2EAssertion,
	) error {
		return h.assertStructuredFieldEquals(scenario, result, assertion)
	},
	e2EAssertionSARIFFieldEquals: func(
		h *e2EHarness,
		scenario e2EScenario,
		result e2EProcessResult,
		assertion e2EAssertion,
	) error {
		return h.assertStructuredFieldEquals(scenario, result, assertion)
	},
}

var (
	e2eBinaryBuildOnce sync.Once
	e2eBinaryPath      string
	e2eBinaryBuildErr  error
	e2eBinaryBuilds    atomic.Int32
)

func newE2EHarness(t *testing.T) *e2EHarness {
	t.Helper()

	binaryPath, err := ensureE2EBinaryBuilt()
	if err != nil {
		t.Fatalf("build e2e binary: %v", err)
	}

	return &e2EHarness{binaryPath: binaryPath}
}

func (h *e2EHarness) run(workDir string, args []string, env map[string]string) (e2EProcessResult, error) {
	cmd := exec.Command(h.binaryPath, args...)
	cmd.Dir = workDir
	cmd.Env = mergeEnv(env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := e2EProcessResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()

		return result, nil
	}

	return e2EProcessResult{}, fmt.Errorf("execute binary %q: %w", h.binaryPath, err)
}

func (h *e2EHarness) mustRunScenario(t *testing.T, scenario e2EScenario) e2EProcessResult {
	t.Helper()

	result, err := h.run(scenario.Fixture, scenario.Command, scenario.Env)
	if err != nil {
		t.Fatalf("%s", h.scenarioFailureDiagnostic(scenario, e2EProcessResult{}, fmt.Sprintf("run scenario: %v", err)))
	}

	if result.ExitCode != scenario.ExpectedExit {
		t.Fatalf(
			"%s",
			h.scenarioFailureDiagnostic(
				scenario,
				result,
				fmt.Sprintf("expected exit code %d, got %d", scenario.ExpectedExit, result.ExitCode),
			),
		)
	}

	h.assertScenarioAssertions(t, scenario, result)

	return result
}

func (h *e2EHarness) assertScenarioAssertions(t *testing.T, scenario e2EScenario, result e2EProcessResult) {
	t.Helper()

	for index, assertion := range scenario.Assertions {
		if err := h.evaluateScenarioAssertion(scenario, result, assertion); err != nil {
			t.Fatalf(
				"%s",
				h.scenarioFailureDiagnostic(
					scenario,
					result,
					fmt.Sprintf("assertion[%d] (%s): %v", index, assertion.Type, err),
				),
			)
		}
	}
}

func (h *e2EHarness) evaluateScenarioAssertion(
	scenario e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	evaluator, ok := e2EAssertionEvaluators[assertion.Type]
	if !ok {
		return fmt.Errorf("unknown assertion type %q", assertion.Type)
	}

	return evaluator(h, scenario, result, assertion)
}

func sortE2EScenarios(input []e2EScenario) []e2EScenario {
	ordered := append([]e2EScenario(nil), input...)

	sort.Slice(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]

		leftTier := e2ETierOrder(left.Tier)
		rightTier := e2ETierOrder(right.Tier)
		if leftTier != rightTier {
			return leftTier < rightTier
		}

		if left.ID != right.ID {
			return left.ID < right.ID
		}

		if left.Name != right.Name {
			return left.Name < right.Name
		}

		if left.Fixture != right.Fixture {
			return left.Fixture < right.Fixture
		}

		return strings.Join(left.Command, "\x00") < strings.Join(right.Command, "\x00")
	})

	return ordered
}

func e2ETierOrder(tier string) int {
	switch tier {
	case "smoke":
		return 0
	case "full":
		return 1
	default:
		return 2
	}
}

func newE2ESmoke001Scenario(moduleRoot string) e2EScenario {
	fixturePath := filepath.Join(moduleRoot, "testdata", "fixtures")
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-SMOKE-001",
		Tier:         "smoke",
		Name:         "analyze happy path deterministic summary",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "Found token token=abc"},
			{Type: e2EAssertionStdoutContains, Value: "sample.txt:1"},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=1 durationMs=[0-9]+$`},
		},
	}
}

func newE2ESmoke002Scenario(moduleRoot string) e2EScenario {
	fixturePath := filepath.Join(moduleRoot, "testdata", "fixtures")
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "missing-config.yaml")

	return e2EScenario{
		ID:           "E2E-SMOKE-002",
		Tier:         "smoke",
		Name:         "invalid config path returns single actionable error",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "."},
		ExpectedExit: 1,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutRegex, Value: `^config file not found: [^\n]+\n?$`},
		},
	}
}

func newE2ESmoke003Scenario(moduleRoot string) e2EScenario {
	fixturePath := filepath.Join(moduleRoot, "testdata", "fixtures")
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "fail.yaml")

	return e2EScenario{
		ID:           "E2E-SMOKE-003",
		Tier:         "smoke",
		Name:         "fail-on threshold exceeded",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "."},
		ExpectedExit: 2,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "Found token token=abc"},
			{Type: e2EAssertionStdoutContains, Value: "sample.txt:1"},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=1 durationMs=[0-9]+$`},
		},
	}
}

func newE2ESmoke004Scenario(moduleRoot string) e2EScenario {
	fixturePath := moduleRoot

	return e2EScenario{
		ID:           "E2E-SMOKE-004",
		Tier:         "smoke",
		Name:         "no findings exits zero",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", "testdata/rules/example.yaml", "testdata/e2e-fixtures/no-findings"},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "No matches found."},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=0 durationMs=[0-9]+$`},
			{Type: e2EAssertionStdoutNotContains, Value: "Found token token="},
		},
	}
}

func newE2ESmoke005Scenario(moduleRoot string) e2EScenario {
	fixturePath := filepath.Join(moduleRoot, "testdata", "fixtures")
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-SMOKE-005",
		Tier:         "smoke",
		Name:         "NO_COLOR disables ANSI output",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "."},
		Env:          map[string]string{"NO_COLOR": "1"},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "Found token token=abc"},
			{Type: e2EAssertionStdoutContains, Value: "sample.txt:1"},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=1 durationMs=[0-9]+$`},
			{Type: e2EAssertionStdoutNotContains, Value: "\x1b["},
		},
	}
}

func newE2ESmoke006Scenario(moduleRoot string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")
	scanPath := filepath.Join("testdata", "e2e-fixtures", "path with spaces")
	reportedPath := filepath.Join(moduleRoot, "testdata", "e2e-fixtures", "path with spaces", "sample file.txt") + ":1"

	return e2EScenario{
		ID:           "E2E-SMOKE-006",
		Tier:         "smoke",
		Name:         "path containing spaces reports correctly",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, scanPath},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "Found token token=abc"},
			{Type: e2EAssertionStdoutContains, Value: "sample file.txt"},
			{Type: e2EAssertionStdoutContains, Value: reportedPath},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=1 durationMs=[0-9]+$`},
		},
	}
}

func newE2EFull001Scenario(moduleRoot string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "fail.yaml")
	baselinePath := filepath.Join(moduleRoot, "testdata", "baseline", "valid-equal.json")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:           "E2E-FULL-001",
		Tier:         "full",
		Name:         "baseline compare suppresses non-regressions",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--baseline", baselinePath, scanPath},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "No matches found."},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=0 durationMs=[0-9]+$`},
			{Type: e2EAssertionStdoutNotContains, Value: "Found token token=abc"},
		},
	}
}

func newE2EFull002Scenario(moduleRoot, baselinePath string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "fail.yaml")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:      "E2E-FULL-002",
		Tier:    "full",
		Name:    "baseline generation overwrites target and exits zero",
		Fixture: fixturePath,
		Command: []string{
			"analyze",
			"--config", configPath,
			"--baseline", baselinePath,
			"--write-baseline",
			"--format", "json",
			scanPath,
		},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionFileExists, Path: baselinePath},
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Path: baselinePath, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Path: baselinePath, Field: "entries.0.filePath", Expected: "sample.txt"},
			{
				Type:     e2EAssertionJSONFieldEquals,
				Path:     baselinePath,
				Field:    "entries.0.message",
				Expected: "Found token token=abc",
			},
			{Type: e2EAssertionJSONFieldEquals, Path: baselinePath, Field: "entries.0.count", Expected: 1},
		},
	}
}

func newE2EFull003Scenario(moduleRoot, baselinePath string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "baseline.yaml")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:      "E2E-FULL-003",
		Tier:    "full",
		Name:    "baseline path precedence prefers --baseline over ruleset baseline",
		Fixture: fixturePath,
		Command: []string{
			"analyze",
			"--config", configPath,
			"--baseline", baselinePath,
			scanPath,
		},
		ExpectedExit: 2,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutContains, Value: "Found token token=abc"},
			{Type: e2EAssertionStdoutContains, Value: "sample.txt:1"},
			{Type: e2EAssertionStdoutRegex, Value: `(?m)^Summary: files=1 skipped=0 matches=1 durationMs=[0-9]+$`},
			{Type: e2EAssertionStdoutNotContains, Value: "No matches found."},
		},
	}
}

func newE2EFull004Scenario(moduleRoot string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:           "E2E-FULL-004",
		Tier:         "full",
		Name:         "json-only format writes to stdout when out path is unset",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", scanPath},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutNotContains, Value: "Summary:"},
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "sample.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.severity", Expected: "error"},
		},
	}
}

func newE2EFull005Scenario(moduleRoot string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:           "E2E-FULL-005",
		Tier:         "full",
		Name:         "sarif-only format writes to stdout when out path is unset",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "sarif", scanPath},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutNotContains, Value: "Summary:"},
			{Type: e2EAssertionSARIFFieldEquals, Field: "$schema", Expected: "https://json.schemastore.org/sarif-2.1.0.json"},
			{Type: e2EAssertionSARIFFieldEquals, Field: "version", Expected: "2.1.0"},
			{Type: e2EAssertionSARIFFieldEquals, Field: "runs.0.columnKind", Expected: "unicodeCodePoints"},
			{Type: e2EAssertionSARIFFieldEquals, Field: "runs.0.results.0.ruleId", Expected: "RC0001"},
			{Type: e2EAssertionSARIFFieldEquals, Field: "runs.0.results.0.level", Expected: "error"},
			{
				Type:     e2EAssertionSARIFFieldEquals,
				Field:    "runs.0.results.0.locations.0.physicalLocation.artifactLocation.uri",
				Expected: "sample.txt",
			},
		},
	}
}

func newE2EFull006Scenario(moduleRoot, outJSONPath string) e2EScenario {
	fixturePath := moduleRoot
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")
	scanPath := filepath.Join("testdata", "fixtures")

	return e2EScenario{
		ID:      "E2E-FULL-006",
		Tier:    "full",
		Name:    "multi-format requires explicit out paths",
		Fixture: fixturePath,
		Command: []string{
			"analyze",
			"--config", configPath,
			"--format", "json,sarif",
			"--out-json", outJSONPath,
			scanPath,
		},
		ExpectedExit: 1,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutRegex, Value: `^--out-sarif is required when requesting sarif with multiple formats\n?$`},
			{Type: e2EAssertionStdoutNotContains, Value: "Summary:"},
			{Type: e2EAssertionFileNotExists, Path: outJSONPath},
		},
	}
}

func newE2EFull007Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-007",
		Tier:         "full",
		Name:         "git mode off applies .gitignore by default when git executable is unavailable",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "--git-mode", "off", "."},
		Env:          map[string]string{"PATH": ""},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesScanned", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesSkipped", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 0},
		},
	}
}

func newE2EFull008Scenario(moduleRoot, repoPath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-008",
		Tier:         "full",
		Name:         "git mode staged scans only staged files",
		Fixture:      repoPath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "--git-mode", "staged", "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "staged.txt"},
		},
	}
}

func newE2EFull009Scenario(moduleRoot, repoPath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:      "E2E-FULL-009",
		Tier:    "full",
		Name:    "git mode diff scans only diff-selected files",
		Fixture: repoPath,
		Command: []string{
			"analyze",
			"--config", configPath,
			"--format", "json",
			"--git-mode", "diff",
			"--git-diff", "HEAD",
			".",
		},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "changed.txt"},
		},
	}
}

func newE2EFull010Scenario(moduleRoot, repoPath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:      "E2E-FULL-010",
		Tier:    "full",
		Name:    "git added lines only reports matches on added lines",
		Fixture: repoPath,
		Command: []string{
			"analyze",
			"--config", configPath,
			"--format", "json",
			"--git-mode", "staged",
			"--git-added-lines-only",
			".",
		},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "sample.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.line", Expected: 3},
		},
	}
}

func newE2EFull011Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-011",
		Tier:         "full",
		Name:         "git enabled run outside repository exits with single error",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--git-mode", "staged", "."},
		ExpectedExit: 1,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutRegex, Value: `^git mode staged requires a git repository\n?$`},
			{Type: e2EAssertionStdoutNotContains, Value: "Summary:"},
		},
	}
}

func newE2EFull012Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-012",
		Tier:         "full",
		Name:         "binary and oversized files are skipped with deterministic stats",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "--max-file-size", "10", "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesScanned", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesSkipped", Expected: 2},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "scannable.txt"},
		},
	}
}

func newE2EFull013Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-013",
		Tier:         "full",
		Name:         "unreadable files are skipped while scan continues",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesScanned", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesSkipped", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "scannable.txt"},
		},
	}
}

func newE2EFull014Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-014",
		Tier:         "full",
		Name:         "ignore precedence favors .reglintignore over .ignore over .gitignore",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "--git-mode", "staged", "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "ignore-wins.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.line", Expected: 1},
			{Type: e2EAssertionStdoutNotContains, Value: `"filePath":"reglintignore-wins.txt"`},
		},
	}
}

func newE2EFull015Scenario(moduleRoot, fixturePath string) e2EScenario {
	configPath := filepath.Join(moduleRoot, "testdata", "rules", "example.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-015",
		Tier:         "full",
		Name:         "repeated runs produce stable ordering",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "."},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 3},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.filePath", Expected: "a.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.line", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.1.filePath", Expected: "a.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.1.line", Expected: 2},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.2.filePath", Expected: "b.txt"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.2.line", Expected: 1},
		},
	}
}

func newE2EFull016Scenario(fixturePath string) e2EScenario {
	configPath := filepath.Join(fixturePath, "reglint.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-016",
		Tier:         "full",
		Name:         "config ruleFiles loads rules from separate files",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--format", "json", "src"},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.filesScanned", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 1},
		},
	}
}

func newE2EFull017Scenario(fixturePath string) e2EScenario {
	configPath := filepath.Join(fixturePath, "reglint.yaml")
	packPath := filepath.Join(fixturePath, "pack.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-017",
		Tier:         "full",
		Name:         "--rules flag loads rule files after config rules",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--rules", packPath, "--format", "json", "src"},
		ExpectedExit: 0,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionJSONFieldEquals, Field: "schemaVersion", Expected: 1},
			{Type: e2EAssertionJSONFieldEquals, Field: "stats.matches", Expected: 2},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.0.message", Expected: "Alpha"},
			{Type: e2EAssertionJSONFieldEquals, Field: "matches.1.message", Expected: "Beta"},
		},
	}
}

func newE2EFull018Scenario(fixturePath string) e2EScenario {
	configPath := filepath.Join(fixturePath, "reglint.yaml")
	badPath := filepath.Join(fixturePath, "bad.yaml")

	return e2EScenario{
		ID:           "E2E-FULL-018",
		Tier:         "full",
		Name:         "rule file with settings exits with single error",
		Fixture:      fixturePath,
		Command:      []string{"analyze", "--config", configPath, "--rules", badPath, "."},
		ExpectedExit: 1,
		Assertions: []e2EAssertion{
			{Type: e2EAssertionStdoutRegex, Value: `^rule file .*only "rules" is allowed in rule files, found "failOn"\n?$`},
		},
	}
}

func assertRegexMatch(value, pattern, streamName string) error {
	if pattern == "" {
		return fmt.Errorf("%s regex pattern is required", streamName)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("compile %s regex %q: %w", streamName, pattern, err)
	}

	if re.MatchString(value) {
		return nil
	}

	return fmt.Errorf("%s does not match regex %q", streamName, pattern)
}

func evalStdoutContains(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertStreamContains(result.Stdout, assertion.Value, "stdout")
}

func evalStderrContains(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertStreamContains(result.Stderr, assertion.Value, "stderr")
}

func evalStdoutNotContains(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertStreamNotContains(result.Stdout, assertion.Value, "stdout")
}

func evalStderrNotContains(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertStreamNotContains(result.Stderr, assertion.Value, "stderr")
}

func evalStdoutRegex(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertRegexMatch(result.Stdout, assertion.Value, "stdout")
}

func evalStderrRegex(
	_ *e2EHarness,
	_ e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	return assertRegexMatch(result.Stderr, assertion.Value, "stderr")
}

func evalFileExists(
	_ *e2EHarness,
	scenario e2EScenario,
	_ e2EProcessResult,
	assertion e2EAssertion,
) error {
	resolvedPath := resolveScenarioPath(scenario, assertion.Path)
	if resolvedPath == "" {
		return errors.New("fileExists requires path")
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		return fmt.Errorf("expected path to exist: %s (%w)", resolvedPath, err)
	}

	return nil
}

func evalFileNotExists(
	_ *e2EHarness,
	scenario e2EScenario,
	_ e2EProcessResult,
	assertion e2EAssertion,
) error {
	resolvedPath := resolveScenarioPath(scenario, assertion.Path)
	if resolvedPath == "" {
		return errors.New("fileNotExists requires path")
	}

	_, err := os.Stat(resolvedPath)
	if err == nil {
		return fmt.Errorf("expected path to be absent: %s", resolvedPath)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("stat path %s: %w", resolvedPath, err)
}

func assertStreamContains(value, expected, streamName string) error {
	if strings.Contains(value, expected) {
		return nil
	}

	return fmt.Errorf("%s does not contain %q", streamName, expected)
}

func assertStreamNotContains(value, blocked, streamName string) error {
	if !strings.Contains(value, blocked) {
		return nil
	}

	return fmt.Errorf("%s unexpectedly contains %q", streamName, blocked)
}

func (h *e2EHarness) assertStructuredFieldEquals(
	scenario e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) error {
	doc, err := h.loadAssertionDocument(scenario, result, assertion)
	if err != nil {
		return err
	}

	fieldValue, err := resolveStructuredField(doc, assertion.Field)
	if err != nil {
		return err
	}

	if valuesEqual(fieldValue, assertion.Expected) {
		return nil
	}

	return fmt.Errorf("field %q mismatch: expected %v, got %v", assertion.Field, assertion.Expected, fieldValue)
}

func (h *e2EHarness) loadAssertionDocument(
	scenario e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) (any, error) {
	data, err := h.loadAssertionPayload(scenario, result, assertion)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode assertion document: %w", err)
	}

	return document, nil
}

func (h *e2EHarness) loadAssertionPayload(
	scenario e2EScenario,
	result e2EProcessResult,
	assertion e2EAssertion,
) ([]byte, error) {
	if assertion.Path != "" {
		resolvedPath := resolveScenarioPath(scenario, assertion.Path)
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("read assertion path %s: %w", resolvedPath, err)
		}

		return data, nil
	}

	if result.Stdout == "" {
		return nil, errors.New("assertion requires JSON payload in stdout when path is unset")
	}

	return []byte(result.Stdout), nil
}

func resolveStructuredField(document any, field string) (any, error) {
	if field == "" {
		return nil, errors.New("field is required")
	}

	current := document
	segments := strings.Split(field, ".")
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, fmt.Errorf("field segment %q not found", segment)
			}

			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil {
				return nil, fmt.Errorf("field segment %q is not a valid array index", segment)
			}
			if index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("array index %d out of range", index)
			}

			current = typed[index]
		default:
			return nil, fmt.Errorf("cannot traverse segment %q on non-container value", segment)
		}
	}

	return current, nil
}

func valuesEqual(got, want any) bool {
	if gotNumber, ok := toFloat64(got); ok {
		if wantNumber, ok := toFloat64(want); ok {
			return gotNumber == wantNumber
		}
	}

	return reflect.DeepEqual(got, want)
}

func toFloat64(value any) (float64, bool) {
	if number, ok := value.(json.Number); ok {
		parsed, err := number.Float64()
		if err != nil {
			return 0, false
		}

		return parsed, true
	}

	rawValue := reflect.ValueOf(value)
	if !rawValue.IsValid() {
		return 0, false
	}

	if isSignedIntegerKind(rawValue.Kind()) {
		return float64(rawValue.Int()), true
	}

	if isUnsignedIntegerKind(rawValue.Kind()) {
		return float64(rawValue.Uint()), true
	}

	if isFloatKind(rawValue.Kind()) {
		return rawValue.Float(), true
	}

	return 0, false
}

func isSignedIntegerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func isUnsignedIntegerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isFloatKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func resolveScenarioPath(scenario e2EScenario, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	if scenario.Fixture == "" {
		return path
	}

	return filepath.Join(scenario.Fixture, path)
}

func (h *e2EHarness) assertScenarioStdoutContains(
	t *testing.T,
	scenario e2EScenario,
	result e2EProcessResult,
	want string,
) {
	t.Helper()

	if strings.Contains(result.Stdout, want) {
		return
	}

	t.Fatalf(
		"%s",
		h.scenarioFailureDiagnostic(
			scenario,
			result,
			fmt.Sprintf("expected stdout to contain %q", want),
		),
	)
}

func (h *e2EHarness) assertScenarioStderrEmpty(t *testing.T, scenario e2EScenario, result e2EProcessResult) {
	t.Helper()

	if result.Stderr == "" {
		return
	}

	t.Fatalf(
		"%s",
		h.scenarioFailureDiagnostic(
			scenario,
			result,
			fmt.Sprintf("expected empty stderr, got %q", result.Stderr),
		),
	)
}

func (h *e2EHarness) assertScenarioPathExists(
	t *testing.T,
	scenario e2EScenario,
	result e2EProcessResult,
	path string,
) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		return
	}

	t.Fatalf(
		"%s",
		h.scenarioFailureDiagnostic(
			scenario,
			result,
			fmt.Sprintf("expected path to exist: %s", path),
		),
	)
}

func (h *e2EHarness) scenarioFailureDiagnostic(scenario e2EScenario, result e2EProcessResult, reason string) string {
	lines := []string{
		"e2e scenario assertion failed:",
		"  reason: " + reason,
		"  scenario: " + scenario.ID,
		"  tier: " + scenario.Tier,
		"  name: " + scenario.Name,
		"  fixture: " + scenario.Fixture,
		"  replay: " + h.scenarioReplayCommand(scenario),
		fmt.Sprintf("  exitCode: %d", result.ExitCode),
		fmt.Sprintf("  stdout: %q", result.Stdout),
		fmt.Sprintf("  stderr: %q", result.Stderr),
	}

	return strings.Join(lines, "\n")
}

func (h *e2EHarness) scenarioReplayCommand(scenario e2EScenario) string {
	envKeys := make([]string, 0, len(scenario.Env))
	for key := range scenario.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	parts := make([]string, 0, 1+len(scenario.Command))
	parts = append(parts, shellQuote(h.binaryPath))
	for _, part := range scenario.Command {
		parts = append(parts, shellQuote(part))
	}
	invocation := strings.Join(parts, " ")

	if len(envKeys) > 0 {
		envPairs := make([]string, 0, len(envKeys))
		for _, key := range envKeys {
			envPairs = append(envPairs, key+"="+shellQuote(scenario.Env[key]))
		}

		invocation = strings.Join(envPairs, " ") + " " + invocation
	}

	if scenario.Fixture == "" {
		return invocation
	}

	return "(cd " + shellQuote(scenario.Fixture) + " && " + invocation + ")"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func e2eBinaryBuildInvocations() int {
	return int(e2eBinaryBuilds.Load())
}

func ensureE2EBinaryBuilt() (string, error) {
	e2eBinaryBuildOnce.Do(func() {
		e2eBinaryBuilds.Add(1)

		moduleRoot, err := findModuleRoot()
		if err != nil {
			e2eBinaryBuildErr = err

			return
		}

		outDir, err := os.MkdirTemp("", "reglint-e2e-bin-")
		if err != nil {
			e2eBinaryBuildErr = fmt.Errorf("create temp e2e build directory: %w", err)

			return
		}

		binaryName := "reglint-e2e"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}

		binaryPath := filepath.Join(outDir, binaryName)

		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/reglint")
		cmd.Dir = moduleRoot

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			e2eBinaryBuildErr = fmt.Errorf(
				"go build ./cmd/reglint failed: %w; stdout=%q stderr=%q",
				err,
				strings.TrimSpace(stdout.String()),
				strings.TrimSpace(stderr.String()),
			)

			return
		}

		e2eBinaryPath = binaryPath
	})

	if e2eBinaryBuildErr != nil {
		return "", e2eBinaryBuildErr
	}

	return e2eBinaryPath, nil
}

func findModuleRoot() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve current file path")
	}

	dir := filepath.Dir(currentFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", currentFile)
		}

		dir = parent
	}
}

func mergeEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}

	merged := map[string]string{}
	for _, pair := range os.Environ() {
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			continue
		}

		merged[pair[:idx]] = pair[idx+1:]
	}

	for key, value := range overrides {
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+merged[key])
	}

	return env
}
