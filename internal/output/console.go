// Package output provides scan result formatters.
package output

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/reglint/reglint/internal/scan"
)

// Console color source markers used for precedence reporting.
const (
	ConsoleColorSourceDefault = "default"
	ConsoleColorSourceConfig  = "config"
	ConsoleColorSourceEnv     = "env"
)

// ConsoleColorSettings defines effective color controls for console output.
type ConsoleColorSettings struct {
	Enabled bool
	Source  string
}

func defaultConsoleColorSettings() ConsoleColorSettings {
	return ConsoleColorSettings{
		Enabled: true,
		Source:  ConsoleColorSourceDefault,
	}
}

func normalizeConsoleColorSettings(settings ConsoleColorSettings) ConsoleColorSettings {
	if settings.Source == "" {
		return defaultConsoleColorSettings()
	}

	return settings
}

// WriteConsole renders a scan result to the provided writer.
func WriteConsole(result scan.Result, out io.Writer) error {
	return writeConsole(result, ConsoleColorSettings{Enabled: false, Source: ConsoleColorSourceConfig}, out)
}

func writeConsole(result scan.Result, settings ConsoleColorSettings, out io.Writer) error {
	matches := append([]scan.Match{}, result.Matches...)
	sort.Slice(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.FilePath != right.FilePath {
			return left.FilePath < right.FilePath
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.Severity != right.Severity {
			return severityRank(left.Severity) < severityRank(right.Severity)
		}
		if left.Message != right.Message {
			return left.Message < right.Message
		}
		if left.Root != right.Root {
			return left.Root < right.Root
		}

		return left.RuleIndex < right.RuleIndex
	})

	var builder strings.Builder
	if err := appendConsoleMatches(&builder, matches, settings.Enabled); err != nil {
		return err
	}

	builder.WriteString(fmt.Sprintf("Summary: files=%d skipped=%d matches=%d durationMs=%d\n",
		result.Stats.FilesScanned,
		result.Stats.FilesSkipped,
		result.Stats.Matches,
		result.Stats.DurationMs,
	))

	_, err := io.WriteString(out, builder.String())

	return err
}

// WriteConsoleWithSettings renders a scan result with explicit color settings.
func WriteConsoleWithSettings(result scan.Result, settings ConsoleColorSettings, out io.Writer) error {
	normalized := normalizeConsoleColorSettings(settings)

	return writeConsole(result, normalized, out)
}

// ConsoleFormatter renders console output.
type ConsoleFormatter struct {
	ColorSettings ConsoleColorSettings
}

// Name returns the format identifier.
func (ConsoleFormatter) Name() string {
	return "console"
}

// Write renders console output to the writer.
func (f ConsoleFormatter) Write(result scan.Result, out io.Writer) error {
	return WriteConsoleWithSettings(result, f.ColorSettings, out)
}

func appendConsoleMatches(builder *strings.Builder, matches []scan.Match, colorsEnabled bool) error {
	if len(matches) == 0 {
		builder.WriteString("No matches found.\n")

		return nil
	}

	currentFile := ""
	for _, match := range matches {
		if match.FilePath != currentFile {
			currentFile = match.FilePath
			builder.WriteString(match.FilePath)
			builder.WriteString("\n")
		}
		line, err := formatConsoleMatchLineWithColor(match, colorsEnabled)
		if err != nil {
			return err
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return nil
}

func formatConsoleMatchLine(match scan.Match) (string, error) {
	return formatConsoleMatchLineWithColor(match, false)
}

func formatConsoleMatchLineWithColor(match scan.Match, colorsEnabled bool) (string, error) {
	absPath, err := absolutePathWithLine(match.FilePath, match.Root, match.Line)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"- %s %d:%d %s\n  %s\n",
		formatSeveritySegment(match.Severity, colorsEnabled),
		match.Line,
		match.Column,
		match.Message,
		absPath,
	), nil
}

func formatSeveritySegment(value string, colorsEnabled bool) string {
	label := severityLabel(value)
	if !colorsEnabled {
		return fmt.Sprintf("%-*s", severityLabelWidth, label)
	}

	colorCode := severityColorCode(value)
	if colorCode == "" {
		return fmt.Sprintf("%-*s", severityLabelWidth, label)
	}

	padding := severityLabelWidth - len(label)
	if padding < 0 {
		padding = 0
	}

	return colorizedSeverityLabel(label, colorCode) + strings.Repeat(" ", padding)
}

func colorizedSeverityLabel(label, colorCode string) string {
	return "\x1b[" + colorCode + "m" + label + "\x1b[0m"
}

func severityColorCode(value string) string {
	switch value {
	case "error":
		return "31"
	case "warning":
		return "33"
	case "notice":
		return "36"
	case "info":
		return "34"
	default:
		return ""
	}
}

func absolutePathWithLine(filePath string, root string, line int) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path required")
	}
	if root == "" {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%s:%d", absPath, line), nil
	}

	fullPath := filepath.Join(root, filepath.FromSlash(filePath))
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", absPath, line), nil
}

const (
	severityRankError = iota
	severityRankWarning
	severityRankNotice
	severityRankInfo
	severityRankUnknown
)

const severityLabelWidth = 5

func severityRank(value string) int {
	switch value {
	case "error":
		return severityRankError
	case "warning":
		return severityRankWarning
	case "notice":
		return severityRankNotice
	case "info":
		return severityRankInfo
	default:
		return severityRankUnknown
	}
}

func severityLabel(value string) string {
	switch value {
	case "error":
		return "ERROR"
	case "warning":
		return "WARN"
	case "notice":
		return "NOTICE"
	case "info":
		return "INFO"
	default:
		return strings.ToUpper(value)
	}
}
