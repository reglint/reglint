package cli

import (
	"bytes"
	"runtime/debug"
	"testing"
)

// The tests below mutate the readBuildInfo seam (package state), so they
// intentionally do not run in parallel.

func TestEffectiveVersionPrefersInjectedVersion(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}, true
	}
	defer func() { readBuildInfo = original }()

	if got := effectiveVersion("0.5.0"); got != "0.5.0" {
		t.Fatalf("expected 0.5.0, got %q", got)
	}
}

func TestEffectiveVersionFallsBackToBuildInfoVersion(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, true
	}
	defer func() { readBuildInfo = original }()

	if got := effectiveVersion("dev"); got != "0.2.0" {
		t.Fatalf("expected 0.2.0, got %q", got)
	}
}

func TestEffectiveVersionStaysDevForDevelBuilds(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	}
	defer func() { readBuildInfo = original }()

	if got := effectiveVersion("dev"); got != "dev" {
		t.Fatalf("expected dev, got %q", got)
	}
}

func TestEffectiveVersionStaysDevWithoutBuildInfo(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	defer func() { readBuildInfo = original }()

	if got := effectiveVersion("dev"); got != "dev" {
		t.Fatalf("expected dev, got %q", got)
	}
}

func TestHandleVersionReportsBuildInfoVersion(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
	}
	defer func() { readBuildInfo = original }()

	var out bytes.Buffer
	if code := HandleVersion(nil, &out); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if got, want := out.String(), "reglint version 1.2.3\n"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
