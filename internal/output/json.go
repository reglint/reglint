package output

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/reglint/reglint/internal/scan"
)

type jsonResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Matches       []jsonMatch `json:"matches"`
	Stats         jsonStats   `json:"stats"`
}

type jsonMatch struct {
	Message      string `json:"message"`
	Severity     string `json:"severity"`
	FilePath     string `json:"filePath"`
	AbsolutePath string `json:"absolutePath"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	MatchText    string `json:"matchText"`
}

type jsonStats struct {
	FilesScanned int   `json:"filesScanned"`
	FilesSkipped int   `json:"filesSkipped"`
	Matches      int   `json:"matches"`
	DurationMs   int64 `json:"durationMs"`
}

// WriteJSON renders a scan result to the provided writer.
func WriteJSON(result scan.Result, out io.Writer) error {
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

		return left.Message < right.Message
	})

	jsonMatches, err := buildJSONMatches(matches)
	if err != nil {
		return err
	}

	payload := jsonResult{
		SchemaVersion: 1,
		Matches:       jsonMatches,
		Stats:         buildJSONStats(result.Stats),
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "\t")

	return encoder.Encode(payload)
}

// JSONFormatter renders JSON output.
type JSONFormatter struct{}

// Name returns the format identifier.
func (JSONFormatter) Name() string {
	return "json"
}

// Write renders JSON output to the writer.
func (JSONFormatter) Write(result scan.Result, out io.Writer) error {
	return WriteJSON(result, out)
}

func buildJSONMatches(matches []scan.Match) ([]jsonMatch, error) {
	if len(matches) == 0 {
		return []jsonMatch{}, nil
	}

	converted := make([]jsonMatch, len(matches))
	for i, match := range matches {
		absolutePath, err := absolutePathWithLine(match.FilePath, match.Root, match.Line)
		if err != nil {
			return nil, err
		}
		converted[i] = jsonMatch{
			Message:      match.Message,
			Severity:     match.Severity,
			FilePath:     match.FilePath,
			AbsolutePath: absolutePath,
			Line:         match.Line,
			Column:       match.Column,
			MatchText:    match.MatchText,
		}
	}

	return converted, nil
}

func buildJSONStats(stats scan.Stats) jsonStats {
	return jsonStats{
		FilesScanned: stats.FilesScanned,
		FilesSkipped: stats.FilesSkipped,
		Matches:      stats.Matches,
		DurationMs:   stats.DurationMs,
	}
}
