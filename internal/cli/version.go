package cli

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"strings"
)

// version is injected at build time via -ldflags; "dev" for local builds.
var version = "dev"

// readBuildInfo is a seam for debug.ReadBuildInfo to keep the fallback testable.
var readBuildInfo = debug.ReadBuildInfo

// HandleVersion prints the build version and always succeeds.
func HandleVersion(_ []string, out *bytes.Buffer) int {
	_, _ = fmt.Fprintf(out, "reglint version %s\n", effectiveVersion(version))

	return 0
}

// effectiveVersion reports the injected version when set, otherwise the module
// version recorded by `go install ...@version`, otherwise "dev".
func effectiveVersion(injected string) string {
	if injected != "dev" {
		return injected
	}

	info, ok := readBuildInfo()
	if !ok {
		return "dev"
	}

	if v := info.Main.Version; v == "" || v == "(devel)" {
		return "dev"
	}

	return strings.TrimPrefix(info.Main.Version, "v")
}
