package output

import (
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	sarifVersion  = "2.1.0"
	sarifSchema   = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifToolName = "RegLint"
)

const (
	ruleIndexMax  = 9999
	ruleIndexPad1 = 10
	ruleIndexPad2 = 100
	ruleIndexPad3 = 1000
)

// WriteSARIF renders a scan result to the provided writer.
func WriteSARIF(result scan.Result, ruleSet []rules.Rule, out io.Writer) error {
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

	report, err := sarif.New(sarif.Version210, true)
	if err != nil {
		return err
	}
	report.Schema = sarifSchema
	run := sarif.NewRun(sarif.Tool{Driver: sarif.NewDriver(sarifToolName)})
	run.ColumnKind = "unicodeCodePoints"

	for i, rule := range ruleSet {
		ruleID := ruleIDForIndex(i)
		sarifRule := sarif.NewRule(ruleID).WithShortDescription(sarif.NewMultiformatMessageString(rule.Message))
		run.Tool.Driver.AddRule(sarifRule)
	}

	for _, match := range matches {
		result := sarif.NewRuleResult(ruleIDForIndex(match.RuleIndex))
		result.WithLevel(sarifLevel(match.Severity))
		result.WithMessage(sarif.NewTextMessage(match.Message))

		region := sarif.NewRegion().WithStartLine(match.Line).WithEndLine(match.Line)
		region.WithStartColumn(match.Column)
		region.WithEndColumn(match.Column + matchTextRuneLength(match.MatchText))
		location := sarif.NewLocation().WithPhysicalLocation(
			sarif.NewPhysicalLocation().
				WithArtifactLocation(sarif.NewSimpleArtifactLocation(normalizePath(match.FilePath))).
				WithRegion(region),
		)
		result.AddLocation(location)
		run.AddResult(result)
	}

	report.AddRun(run)

	return report.PrettyWrite(out)
}

// SARIFFormatter renders SARIF output.
type SARIFFormatter struct {
	Rules []rules.Rule
}

// Name returns the format identifier.
func (SARIFFormatter) Name() string {
	return "sarif"
}

// Write renders SARIF output to the writer.
func (formatter SARIFFormatter) Write(result scan.Result, out io.Writer) error {
	return WriteSARIF(result, formatter.Rules, out)
}

func normalizePath(path string) string {
	cleaned := filepath.ToSlash(path)

	return strings.TrimPrefix(cleaned, "./")
}

func ruleIDForIndex(index int) string {
	return "RC" + formatRuleIndex(index+1)
}

func formatRuleIndex(index int) string {
	value := index
	if value < 0 {
		value = 0
	}
	if value > ruleIndexMax {
		value = ruleIndexMax
	}

	return fmtRuleIndex(value)
}

func fmtRuleIndex(value int) string {
	if value < ruleIndexPad1 {
		return "000" + itoa(value)
	}
	if value < ruleIndexPad2 {
		return "00" + itoa(value)
	}
	if value < ruleIndexPad3 {
		return "0" + itoa(value)
	}

	return itoa(value)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func sarifLevel(severity string) string {
	switch severity {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "notice", "info":
		return "note"
	default:
		return "note"
	}
}

func matchTextRuneLength(text string) int {
	if text == "" {
		return 0
	}

	return utf8.RuneCountInString(text)
}
