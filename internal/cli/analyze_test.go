package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/cli"
)

func TestParseAnalyzeDefaults(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDefaultConfig(t, got, configPath)
}

func TestParseAnalyzeShortConfigFlag(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"-c", configPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDefaultConfig(t, got, configPath)
}

func TestParseAnalyzeShortFormatFlag(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"-c", configPath, "-f", "json"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(got.Formats) != 1 || got.Formats[0] != "json" {
		t.Fatalf("expected formats [json], got %v", got.Formats)
	}
}

func TestParseAnalyzeShortFormatHonorsShortValueWithLongDefault(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"-c", configPath, "-f", "sarif", "--format", "console"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(got.Formats) != 1 || got.Formats[0] != "console" {
		t.Fatalf("expected formats [console], got %v", got.Formats)
	}
}

func TestParseAnalyzeFormatsDedupAndTrim(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json, json"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{"json"}
	if len(got.Formats) != len(want) {
		t.Fatalf("expected formats %v, got %v", want, got.Formats)
	}
	for i := range want {
		if got.Formats[i] != want[i] {
			t.Fatalf("expected formats %v, got %v", want, got.Formats)
		}
	}
}

func TestParseAnalyzeFormatsKeepsOrderAfterDuplicates(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	jsonPath := writableFile(t, "scan.json")
	sarifPath := writableFile(t, "scan.sarif")

	formatValue := strings.Join([]string{"json", "json", "sarif"}, ", ")
	got, err := cli.ParseAnalyzeArgs([]string{
		"--config", configPath,
		"--format", formatValue,
		"--out-json", jsonPath,
		"--out-sarif", sarifPath,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{"json", "sarif"}
	if len(got.Formats) != len(want) {
		t.Fatalf("expected formats %v, got %v", want, got.Formats)
	}
	for i := range want {
		if got.Formats[i] != want[i] {
			t.Fatalf("expected formats %v, got %v", want, got.Formats)
		}
	}
}

func TestParseAnalyzeIncludeExclude(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{
		"--config", configPath,
		"--include", "**/*.go",
		"--include", "**/*.md",
		"--exclude", "**/vendor/**",
		"--exclude", "**/.git/**",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(got.Include) != 2 || got.Include[0] != "**/*.go" || got.Include[1] != "**/*.md" {
		t.Fatalf("unexpected include list: %v", got.Include)
	}
	if len(got.Exclude) != 2 || got.Exclude[0] != "**/vendor/**" || got.Exclude[1] != "**/.git/**" {
		t.Fatalf("unexpected exclude list: %v", got.Exclude)
	}
}

func TestParseAnalyzeInvalidFormat(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "bogus"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeInvalidFailOn(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--fail-on", "fatal"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeNoIgnoreFilesFlag(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--no-ignore-files"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !got.NoIgnoreFiles {
		t.Fatal("expected ignore files to be disabled")
	}
}

func TestParseAnalyzeBaselineFlags(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{
		"--config", configPath,
		"--baseline", "testdata/baseline.json",
		"--write-baseline",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.BaselinePath != "testdata/baseline.json" {
		t.Fatalf("expected baseline path %q, got %q", "testdata/baseline.json", got.BaselinePath)
	}
	if !got.WriteBaseline {
		t.Fatal("expected write-baseline to be true")
	}
}

func TestParseAnalyzeGitDefaults(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.GitMode != "off" {
		t.Fatalf("expected default git mode off, got %q", got.GitMode)
	}
	if got.GitDiffTarget != "" {
		t.Fatalf("expected empty git diff target, got %q", got.GitDiffTarget)
	}
	if got.GitAddedLinesOnly {
		t.Fatal("expected git-added-lines-only to be false")
	}
	if !got.GitignoreEnabled {
		t.Fatal("expected gitignore to be enabled by default")
	}
}

func TestParseAnalyzeGitFlags(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{
		"--config", configPath,
		"--git-mode", "diff",
		"--git-diff", "HEAD~1..HEAD",
		"--git-added-lines-only",
		"--no-gitignore",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.GitMode != "diff" {
		t.Fatalf("expected git mode diff, got %q", got.GitMode)
	}
	if got.GitDiffTarget != "HEAD~1..HEAD" {
		t.Fatalf("expected git diff target HEAD~1..HEAD, got %q", got.GitDiffTarget)
	}
	if !got.GitAddedLinesOnly {
		t.Fatal("expected git-added-lines-only to be true")
	}
	if got.GitignoreEnabled {
		t.Fatal("expected gitignore to be disabled")
	}
}

func TestParseAnalyzeRejectsInvalidGitMode(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--git-mode", "bogus"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeGitDiffImpliesDiffMode(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--git-diff", "HEAD~1..HEAD"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.GitMode != "diff" {
		t.Fatalf("expected git mode diff, got %q", got.GitMode)
	}
}

func TestParseAnalyzeRequiresOutPathForMultiFormat(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "console,json"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRequiresOutSARIFForMultiFormat(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "console,sarif"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeAllowsSingleJsonToStdout(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseAnalyzeAllowsSingleSarifToStdout(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func assertDefaultConfig(t *testing.T, got cli.Config, configPath string) {
	t.Helper()

	if got.ConfigPath != configPath {
		t.Fatalf("expected config path %q, got %q", configPath, got.ConfigPath)
	}
	assertDefaultRoots(t, got.Roots)
	assertDefaultFormats(t, got.Formats)
	if got.Concurrency != runtime.GOMAXPROCS(0) {
		t.Fatalf("expected concurrency %d, got %d", runtime.GOMAXPROCS(0), got.Concurrency)
	}
	if got.MaxFileSizeBytes != 5242880 {
		t.Fatalf("expected max file size 5242880, got %d", got.MaxFileSizeBytes)
	}
	assertDefaultOutputPaths(t, got)
	assertDefaultBaselineSettings(t, got)
}

func assertDefaultRoots(t *testing.T, roots []string) {
	t.Helper()

	if len(roots) != 1 {
		t.Fatalf("expected default root '.', got %v", roots)
	}
	if roots[0] != "." {
		t.Fatalf("expected default root '.', got %v", roots)
	}
}

func assertDefaultFormats(t *testing.T, formats []string) {
	t.Helper()

	if len(formats) != 1 {
		t.Fatalf("expected default format [console], got %v", formats)
	}
	if formats[0] != "console" {
		t.Fatalf("expected default format [console], got %v", formats)
	}
}

func assertDefaultOutputPaths(t *testing.T, got cli.Config) {
	t.Helper()

	if got.OutJSON != "" {
		t.Fatalf("expected empty output paths, got out-json=%q out-sarif=%q", got.OutJSON, got.OutSARIF)
	}
	if got.OutSARIF != "" {
		t.Fatalf("expected empty output paths, got out-json=%q out-sarif=%q", got.OutJSON, got.OutSARIF)
	}
}

func assertDefaultBaselineSettings(t *testing.T, got cli.Config) {
	t.Helper()

	if got.BaselinePath != "" {
		t.Fatalf("expected empty baseline path, got %q", got.BaselinePath)
	}
	if got.RuleSetBaselinePath != "" {
		t.Fatalf("expected empty ruleset baseline path, got %q", got.RuleSetBaselinePath)
	}
	if got.EffectiveBaselinePath != "" {
		t.Fatalf("expected empty effective baseline path, got %q", got.EffectiveBaselinePath)
	}
	if got.WriteBaseline {
		t.Fatal("expected write-baseline to be false")
	}
}

func TestParseAnalyzeRejectsEmptyFormatValue(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", ""})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsWhitespaceFormatValue(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", " "})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsFormatWithEmptyComponent(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json,"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsFormatWithWhitespaceComponent(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json, , sarif"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsEmptyFormatWhenFlagProvided(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", ""})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRequiresConfigFile(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", missingPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsConfigDirectoryEvenWithoutOutPaths(t *testing.T) {
	t.Parallel()

	_, err := cli.ParseAnalyzeArgs([]string{"--config", t.TempDir(), "--format", "json"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsConfigDirectory(t *testing.T) {
	t.Parallel()

	_, err := cli.ParseAnalyzeArgs([]string{"--config", t.TempDir()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsUnreadableConfigFile(t *testing.T) {
	t.Parallel()

	path := writableFile(t, "rules.yaml")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("failed to set permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o600)
	})

	_, err := cli.ParseAnalyzeArgs([]string{"--config", path})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsZeroConcurrency(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--concurrency", "0"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsNegativeConcurrency(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--concurrency", "-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeSetsConcurrencyWhenProvided(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--concurrency", "2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !got.ConcurrencySet {
		t.Fatal("expected concurrency to be marked as set")
	}
}

func TestParseAnalyzeAcceptsConcurrencyOne(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--concurrency", "1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseAnalyzeDefaultConcurrencyUsesGOMAXPROCS(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	got, err := cli.ParseAnalyzeArgs([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Concurrency != runtime.GOMAXPROCS(0) {
		t.Fatalf("expected concurrency %d, got %d", runtime.GOMAXPROCS(0), got.Concurrency)
	}
}

func TestParseAnalyzeRejectsZeroMaxFileSize(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--max-file-size", "0"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeAcceptsMaxFileSizeOne(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--max-file-size", "1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseAnalyzeRejectsUnwritableOutJSON(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputDir := readOnlyDir(t)
	outputPath := filepath.Join(outputDir, "scan.json")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsUnwritableOutSARIF(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputDir := readOnlyDir(t)
	outputPath := filepath.Join(outputDir, "scan.sarif")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeAllowsWritableOutJSONFile(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := writableFile(t, "scan.json")

	_, parseErr := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if parseErr != nil {
		t.Fatalf("expected no error, got %v", parseErr)
	}
}

func TestParseAnalyzeRejectsReadOnlyOutJSONFile(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := readOnlyFile(t, "scan.json")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsOutJSONWithMissingParent(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := filepath.Join(t.TempDir(), "missing", "scan.json")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsOutJSONDirectory(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := t.TempDir()

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsOutJSONWhenParentUnreadable(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputDir := noExecDir(t)
	outputPath := filepath.Join(outputDir, "scan.json")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeAllowsWritableOutSARIFFile(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := writableFile(t, "scan.sarif")

	_, parseErr := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if parseErr != nil {
		t.Fatalf("expected no error, got %v", parseErr)
	}
}

func TestParseAnalyzeRejectsOutSARIFDirectory(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := t.TempDir()

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeRejectsOutSARIFWithMissingParent(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := filepath.Join(t.TempDir(), "missing", "scan.sarif")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAnalyzeAcceptsOutJSONInCurrentDir(t *testing.T) {
	configPath := writeTempConfig(t)
	outputPath := "scan-parse-json.tmp"
	cleanupFile(t, outputPath)
	t.Cleanup(func() {
		cleanupFile(t, outputPath)
	})

	_, parseErr := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if parseErr != nil {
		t.Fatalf("expected no error, got %v", parseErr)
	}
}

func TestParseAnalyzeAcceptsOutSARIFInCurrentDir(t *testing.T) {
	configPath := writeTempConfig(t)
	outputPath := "scan-parse-sarif.tmp"
	cleanupFile(t, outputPath)
	t.Cleanup(func() {
		cleanupFile(t, outputPath)
	})

	_, parseErr := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if parseErr != nil {
		t.Fatalf("expected no error, got %v", parseErr)
	}
}

func TestParseAnalyzeAllowsNewOutJSONInWritableDir(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := filepath.Join(t.TempDir(), "new.json")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "json", "--out-json", outputPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseAnalyzeAllowsNewOutSARIFInWritableDir(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := filepath.Join(t.TempDir(), "new.sarif")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseAnalyzeRejectsReadOnlyOutSARIFFile(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t)
	outputPath := readOnlyFile(t, "scan.sarif")

	_, err := cli.ParseAnalyzeArgs([]string{"--config", configPath, "--format", "sarif", "--out-sarif", outputPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func writeTempConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte("rules: []"), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	return path
}

func readOnlyDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("failed to set read-only permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	return dir
}

func noExecDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o600); err != nil {
		t.Fatalf("failed to set permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	return dir
}

func writableFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	return path
}

func readOnlyFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatalf("failed to set read-only permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o600)
	})

	return path
}

func cleanupFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			t.Fatalf("failed to remove file: %v", err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat file: %v", err)
	}
}
