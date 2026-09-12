package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iyaki/reglint/internal/cli"
)

func TestRunRoutesAnalyzeAlias(t *testing.T) {
	t.Parallel()

	assertAnalyzeRouting(t, "analyse")
}

func TestRunShowsHelpWhenNoCommand(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{}, map[string]cli.Handler{}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	text := output.String()
	if !strings.Contains(text, "Usage:") {
		t.Fatalf("expected help output to include Usage, got %q", text)
	}
}

func TestRunShowsHelpForRootFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"--help"}, map[string]cli.Handler{}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	got := output.String()
	want := "Usage:\n" +
		"  reglint <command> [flags]\n" +
		"\n" +
		"Commands:\n" +
		"  analyze (alias: analyse)\n" +
		"  init\n" +
		"  version\n" +
		"\n" +
		"Flags:\n" +
		"  -h, --help bool (default false)  Print help and exit.\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"bogus"}, map[string]cli.Handler{}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	got := output.String()
	want := "Unknown command: bogus\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunUnknownCommandWithHelpFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"bogus", "--help"}, map[string]cli.Handler{}, &output)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	got := output.String()
	want := "Unknown command: bogus\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunRoutesAnalyzeCommand(t *testing.T) {
	t.Parallel()

	assertAnalyzeRouting(t, "analyze")
}

func TestRunShowsHelpForAnalyzeFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"analyze", "--help"}, map[string]cli.Handler{}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	got := output.String()
	if got != expectedAnalyzeHelpOutput() {
		t.Fatalf("expected %q, got %q", expectedAnalyzeHelpOutput(), got)
	}
}

func TestRunShowsHelpForAnalyseFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"analyse", "-h"}, map[string]cli.Handler{}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	got := output.String()
	if got != expectedAnalyzeHelpOutput() {
		t.Fatalf("expected %q, got %q", expectedAnalyzeHelpOutput(), got)
	}
}

func TestRunShowsHelpForInitFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	code := cli.Run([]string{"init", "-h"}, map[string]cli.Handler{}, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	got := output.String()
	want := "Usage:\n" +
		"  reglint init [flags]\n" +
		"\n" +
		"Flags:\n" +
		"  -h, --help bool (default false)  Print help and exit.\n" +
		"      --out string (default reglint-rules.yaml)  Output path for the config file.\n" +
		"      --force bool (default false)  Overwrite existing config file.\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func assertAnalyzeRouting(t *testing.T, command string) {
	t.Helper()

	var called int
	var gotArgs []string
	handler := func(args []string, out *bytes.Buffer) int {
		called++
		gotArgs = append([]string{}, args...)
		_, _ = out.WriteString("ok")

		return 0
	}

	handlers := map[string]cli.Handler{
		"analyze": func(args []string, out *bytes.Buffer) int {
			return handler(args, out)
		},
	}

	var output bytes.Buffer
	code := cli.Run([]string{command, "./path"}, handlers, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	} else if called != 1 {
		t.Fatalf("expected handler called once, got %d", called)
	} else if len(gotArgs) != 1 || gotArgs[0] != "./path" {
		t.Fatalf("expected args [./path], got %v", gotArgs)
	}
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
