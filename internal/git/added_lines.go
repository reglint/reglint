package git

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

const expectedHunkHeaderMatches = 3

type addedLinesParseState struct {
	currentFilePath  string
	addedLinesByFile map[string]map[int]struct{}
}

// SelectAddedLines resolves added-line sets for Git-scoped scans.
func SelectAddedLines(request CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
	mode := strings.TrimSpace(request.Mode)
	if mode == "" || mode == "off" {
		return nil, nil
	}

	output, err := runAddedLinesCommand(request, mode)
	if err != nil {
		return nil, err
	}

	addedLinesByFile, err := parseAddedLines(output)
	if err != nil {
		switch mode {
		case "staged":
			return nil, fmt.Errorf("git mode staged failed to resolve added lines: %w", err)
		case "diff":
			target := strings.TrimSpace(request.DiffTarget)

			return nil, fmt.Errorf("git mode diff failed to resolve added lines for target %q: %w", target, err)
		default:
			return nil, fmt.Errorf("git mode %s is not supported", mode)
		}
	}

	return addedLinesByFile, nil
}

func runAddedLinesCommand(request CandidateSelectionRequest, mode string) (string, error) {
	switch mode {
	case "staged":
		output, err := runCommand(
			request.WorkingDir,
			"diff",
			"--cached",
			"--unified=0",
			"--no-color",
			"--no-prefix",
			"--diff-filter=ACMR",
		)
		if err != nil {
			return "", errors.New("git mode staged failed to resolve added lines")
		}

		return output, nil
	case "diff":
		target := strings.TrimSpace(request.DiffTarget)
		if target == "" {
			return "", errors.New("git mode diff requires diff target")
		}

		output, err := runCommand(
			request.WorkingDir,
			"diff",
			"--unified=0",
			"--no-color",
			"--no-prefix",
			"--diff-filter=ACMR",
			target,
		)
		if err != nil {
			return "", fmt.Errorf("git mode diff failed to resolve added lines for target %q", target)
		}

		return output, nil
	default:
		return "", fmt.Errorf("git mode %s is not supported", mode)
	}
}

func parseAddedLines(diffOutput string) (map[string]map[int]struct{}, error) {
	state := addedLinesParseState{addedLinesByFile: make(map[string]map[int]struct{})}

	// ponytail: diff output is already fully in memory, so split instead of
	// bufio.Scanner — no per-line cap to trip on (Scanner's 64 KiB default
	// killed diffs containing minified one-line files).
	for _, rawLine := range strings.Split(diffOutput, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if err := state.consumeLine(line); err != nil {
			return nil, err
		}
	}

	return state.finalize(), nil
}

func (state *addedLinesParseState) consumeLine(line string) error {
	switch {
	case strings.HasPrefix(line, "+++ "):
		return state.consumeTargetLine(line)
	case strings.HasPrefix(line, "@@ "):
		return state.consumeHunkLine(line)
	default:
		return nil
	}
}

func (state *addedLinesParseState) consumeTargetLine(line string) error {
	filePath, err := parseDiffTargetPath(line)
	if err != nil {
		return err
	}

	state.currentFilePath = filePath
	if state.currentFilePath != "" {
		if _, ok := state.addedLinesByFile[state.currentFilePath]; !ok {
			state.addedLinesByFile[state.currentFilePath] = make(map[int]struct{})
		}
	}

	return nil
}

func (state *addedLinesParseState) consumeHunkLine(line string) error {
	if state.currentFilePath == "" {
		return errors.New("git diff hunk missing target file path")
	}

	lineStart, lineCount, err := parseHunkHeader(line)
	if err != nil {
		return err
	}
	if lineCount <= 0 {
		return nil
	}

	lineSet := state.addedLinesByFile[state.currentFilePath]
	for lineNumber := lineStart; lineNumber < lineStart+lineCount; lineNumber++ {
		lineSet[lineNumber] = struct{}{}
	}

	return nil
}

func (state *addedLinesParseState) finalize() map[string]map[int]struct{} {
	for filePath, lineSet := range state.addedLinesByFile {
		if len(lineSet) == 0 {
			delete(state.addedLinesByFile, filePath)
		}
	}

	return state.addedLinesByFile
}

func parseDiffTargetPath(line string) (string, error) {
	rawPath := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
	if rawPath == "/dev/null" {
		return "", nil
	}

	rawPath = strings.TrimPrefix(rawPath, "b/")

	return normalizeRelativePath(rawPath)
}

func parseHunkHeader(header string) (int, int, error) {
	matches := hunkHeaderPattern.FindStringSubmatch(header)
	if len(matches) != expectedHunkHeaderMatches {
		return 0, 0, fmt.Errorf("invalid git diff hunk header %q", header)
	}

	lineStart, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid git diff hunk header %q", header)
	}

	lineCount := 1
	if matches[2] != "" {
		lineCount, err = strconv.Atoi(matches[2])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid git diff hunk header %q", header)
		}
	}

	return lineStart, lineCount, nil
}
