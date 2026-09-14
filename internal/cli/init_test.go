package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/cli"
)

func TestParseInitDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := cli.ParseInitArgs([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.OutputPath != "reglint-rules.yaml" {
		t.Fatalf("expected default output path, got %q", cfg.OutputPath)
	}
	if cfg.Force {
		t.Fatalf("expected force to be false")
	}
}

func TestHandleInitWritesDefaultConfig(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "rules.yaml")
	var output bytes.Buffer

	code := cli.HandleInit([]string{"--out", outputPath}, &output)
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
	if string(data) != expectedDefaultConfig() {
		t.Fatalf("unexpected config contents: %q", string(data))
	}
}

func TestHandleInitFailsWhenFileExistsWithoutForce(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(outputPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	var output bytes.Buffer
	code := cli.HandleInit([]string{"--out", outputPath}, &output)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "--force") {
		t.Fatalf("expected force hint, got %q", output.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read existing file: %v", err)
	}
	if string(data) != "existing" {
		t.Fatalf("expected file to remain unchanged, got %q", string(data))
	}
}

func TestHandleInitOverwritesWithForce(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(outputPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	var output bytes.Buffer
	code := cli.HandleInit([]string{"--out", outputPath, "--force"}, &output)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(data) != expectedDefaultConfig() {
		t.Fatalf("unexpected config contents: %q", string(data))
	}
}

func expectedDefaultConfig() string {
	return "" +
		"include:\n" +
		"  - \"**/*\"\n" +
		"exclude:\n" +
		"  - \"**/.git/**\"\n" +
		"  - \"**/node_modules/**\"\n" +
		"  - \"**/vendor/**\"\n" +
		"consoleColorsEnabled: true\n" +
		"failOn: \"error\"\n" +
		"rules:\n" +
		"  - message: \"Avoid hardcoded token: $1\"\n" +
		"    regex: \"token\\s*[:=]\\s*([A-Za-z0-9_-]+)\"\n" +
		"    severity: \"error\"\n" +
		"    paths:\n" +
		"      - \"src/**\"\n"
}
