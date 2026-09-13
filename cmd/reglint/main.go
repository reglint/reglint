// Command reglint provides the CLI entrypoint.
package main

import (
	"io"
	"os"

	"github.com/reglint/reglint/internal/cli"
)

var (
	args                   = os.Args
	outputWriter io.Writer = os.Stdout
	exitFunc               = os.Exit
)

func main() {
	code := run(args[1:], outputWriter)
	exitFunc(code)
}

func run(args []string, out io.Writer) int {
	handlers := map[string]cli.Handler{
		"analyze": cli.HandleAnalyze,
		"init":    cli.HandleInit,
		"version": cli.HandleVersion,
	}

	return cli.Run(args, handlers, out)
}
