package output

import (
	"io"

	"github.com/reglint/reglint/internal/scan"
)

// Formatter renders a scan result to the provided writer.
type Formatter interface {
	Name() string
	Write(result scan.Result, out io.Writer) error
}
