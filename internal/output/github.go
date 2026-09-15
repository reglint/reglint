package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

// githubAnnotationsPerLevel mirrors GitHub's per-step annotation limits.
const githubAnnotationsPerLevel = 10

// WriteGitHub renders a scan result as GitHub workflow command lines.
func WriteGitHub(result scan.Result, _ []rules.Rule, out io.Writer) error {
	matches := append([]scan.Match{}, result.Matches...)
	// ponytail: 4th inline copy of the shared sort comparator; consolidate if a 5th formatter needs it
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

	shownByLevel := map[string]int{}
	dropped := false
	var builder strings.Builder
	for _, match := range matches {
		level := githubLevel(match.Severity)
		if shownByLevel[level] == githubAnnotationsPerLevel {
			dropped = true

			continue
		}
		shownByLevel[level]++
		fmt.Fprintf(&builder, "::%s file=%s,line=%d,col=%d,title=%s::%s\n",
			level,
			escapeGitHubProperty(normalizePath(match.FilePath)),
			match.Line,
			match.Column,
			escapeGitHubProperty(ruleIDForIndex(match.RuleIndex)),
			escapeGitHubMessage(match.Message))
	}

	if dropped {
		shown := shownByLevel["error"] + shownByLevel["warning"] + shownByLevel["notice"]
		fmt.Fprintf(&builder,
			"::notice title=RegLint::%d of %d matches shown; "+
				"GitHub limits annotations to 10 per severity per step. "+
				"Use --format json or sarif for the full list.\n",
			shown, len(matches))
	}

	_, err := io.WriteString(out, builder.String())

	return err
}

// GitHubFormatter renders GitHub workflow command output.
type GitHubFormatter struct {
	Rules []rules.Rule
}

// Name returns the format identifier.
func (GitHubFormatter) Name() string {
	return "github"
}

// Write renders GitHub workflow command output to the writer.
func (formatter GitHubFormatter) Write(result scan.Result, out io.Writer) error {
	return WriteGitHub(result, formatter.Rules, out)
}

func githubLevel(severity string) string {
	switch severity {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "notice"
	}
}

func escapeGitHubMessage(message string) string {
	return strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D").Replace(message)
}

func escapeGitHubProperty(value string) string {
	return strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D", ":", "%3A", ",", "%2C").Replace(value)
}
