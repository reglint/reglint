// Package cli provides command routing for the CLI.
package cli

import (
	"bytes"
	"fmt"
	"io"
)

// Handler handles a CLI subcommand.
type Handler func(args []string, out *bytes.Buffer) int

// Run routes CLI args to the matching handler.
func Run(args []string, handlers map[string]Handler, out io.Writer) int {
	if len(args) == 0 {
		writeHelpTopic(out, rootHelpTopic())

		return 1
	}
	if isHelpArg(args[0]) {
		writeHelpTopic(out, rootHelpTopic())

		return 0
	}

	command := args[0]
	if command == "analyse" {
		command = "analyze"
	}

	commandArgs := args[1:]
	if isHelpRequest(command, commandArgs) {
		topic, ok := getHelpTopic(command)
		if ok {
			writeHelpTopic(out, topic)

			return 0
		}
	}

	handler, ok := handlers[command]
	if !ok {
		_, _ = fmt.Fprintf(out, "Unknown command: %s\n", args[0])

		return 1
	}

	buffer := &bytes.Buffer{}
	code := handler(commandArgs, buffer)
	_, _ = out.Write(buffer.Bytes())

	return code
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h"
}

func isHelpRequest(command string, args []string) bool {
	if command != "analyze" && command != "init" && command != "version" {
		return false
	}
	for _, arg := range args {
		if isHelpArg(arg) {
			return true
		}
	}

	return false
}
