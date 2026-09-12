package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iyaki/reglint/internal/cli"
)

func TestRunRoutesVersionCommand(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	handlers := map[string]cli.Handler{"version": cli.HandleVersion}
	code := cli.Run([]string{"version"}, handlers, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if got, want := output.String(), "reglint version dev\n"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunShowsHelpForVersionFlag(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	handlers := map[string]cli.Handler{"version": cli.HandleVersion}
	code := cli.Run([]string{"version", "--help"}, handlers, &output)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	got := output.String()
	want := "Usage:\n" +
		"  reglint version\n" +
		"\n" +
		"Flags:\n" +
		"  -h, --help bool (default false)  Print help and exit.\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if strings.Contains(got, "dev") {
		t.Fatalf("expected version help topic without version string, got %q", got)
	}
}
