package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/cli"
)

func TestParseValidateArgsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := cli.ParseValidateArgs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConfigPath != "reglint-rules.yaml" {
		t.Fatalf("expected default config path, got %q", cfg.ConfigPath)
	}
	if cfg.RuleFilePaths != nil {
		t.Fatalf("expected nil rule file paths, got %v", cfg.RuleFilePaths)
	}
}

func TestParseValidateArgsFlags(t *testing.T) {
	t.Parallel()

	cfg, err := cli.ParseValidateArgs([]string{"-c", "other.yaml", "--rules", "a.yaml", "--rules", "b.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConfigPath != "other.yaml" {
		t.Fatalf("expected config path other.yaml, got %q", cfg.ConfigPath)
	}
	if len(cfg.RuleFilePaths) != 2 || cfg.RuleFilePaths[0] != "a.yaml" || cfg.RuleFilePaths[1] != "b.yaml" {
		t.Fatalf("expected [a.yaml b.yaml], got %v", cfg.RuleFilePaths)
	}
}

func TestHandleValidateValidConfigExitsZero(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "rules.yaml")
	validConfig := "" +
		"rules:\n" +
		"  - message: \"first\"\n" +
		"    regex: \"alpha\"\n" +
		"  - message: \"second\"\n" +
		"    regex: \"beta\"\n"
	if err := os.WriteFile(configPath, []byte(validConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var output bytes.Buffer
	code := cli.HandleValidate([]string{"-c", configPath}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, output: %q", code, output.String())
	}
	expected := "Config OK: " + configPath + " (2 rules)\n"
	if output.String() != expected {
		t.Fatalf("expected %q, got %q", expected, output.String())
	}
}

func TestHandleValidateInvalidRegexExitsOne(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "rules.yaml")
	invalidConfig := "" +
		"rules:\n" +
		"  - message: \"broken\"\n" +
		"    regex: \"[\"\n"
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var output bytes.Buffer
	code := cli.HandleValidate([]string{"-c", configPath}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d, output: %q", code, output.String())
	}
	if !strings.Contains(output.String(), "regex is invalid") {
		t.Fatalf("expected regex error in output, got %q", output.String())
	}
}

func TestHandleValidateMissingConfigExitsOne(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "missing.yaml")

	var output bytes.Buffer
	code := cli.HandleValidate([]string{"-c", configPath}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d, output: %q", code, output.String())
	}
	if !strings.Contains(output.String(), "config file not found") {
		t.Fatalf("expected config file not found in output, got %q", output.String())
	}
}

func TestHandleValidateRuleFileErrorExitsOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "rules.yaml")
	validConfig := "rules:\n  - message: \"ok\"\n    regex: \"fine\"\n"
	if err := os.WriteFile(configPath, []byte(validConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	rulePackPath := filepath.Join(dir, "pack.yaml")
	if err := os.WriteFile(rulePackPath, []byte("failOn: error\n"), 0o600); err != nil {
		t.Fatalf("write rule pack: %v", err)
	}

	var output bytes.Buffer
	code := cli.HandleValidate([]string{"-c", configPath, "--rules", rulePackPath}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d, output: %q", code, output.String())
	}
	if !strings.Contains(output.String(), "failOn") {
		t.Fatalf("expected failOn in output, got %q", output.String())
	}
}
