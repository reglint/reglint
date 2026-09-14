package cli

import (
	"bytes"
	"fmt"
)

// version is injected at build time via -ldflags; "dev" for local builds.
var version = "dev"

// HandleVersion prints the build version and always succeeds.
func HandleVersion(_ []string, out *bytes.Buffer) int {
	_, _ = fmt.Fprintf(out, "reglint version %s\n", version)

	return 0
}
