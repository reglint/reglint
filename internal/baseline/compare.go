package baseline

import (
	"cmp"
	"sort"

	"github.com/reglint/reglint/internal/scan"
)

type baselineKey struct {
	filePath string
	message  string
}

const (
	severityRankError = iota
	severityRankWarning
	severityRankNotice
	severityRankInfo
	severityRankUnknown
)

// Compare suppresses matches covered by the baseline and returns regressions.
func Compare(current []scan.Match, document Document) Comparison {
	remaining := buildRemainingCounts(document.Entries)
	sorted := sortCurrentMatches(current)
	regressions, suppressedCount := suppressMatches(sorted, remaining)
	improvementsCount := countImprovements(remaining)

	return Comparison{
		Regressions:       regressions,
		SuppressedCount:   suppressedCount,
		ImprovementsCount: improvementsCount,
	}
}

func buildRemainingCounts(entries []Entry) map[baselineKey]int {
	remaining := make(map[baselineKey]int, len(entries))
	for _, entry := range entries {
		key := baselineKey{filePath: entry.FilePath, message: entry.Message}
		remaining[key] = entry.Count
	}

	return remaining
}

func sortCurrentMatches(current []scan.Match) []scan.Match {
	sorted := append([]scan.Match{}, current...)
	sort.Slice(sorted, func(i, j int) bool {
		return compareMatch(sorted[i], sorted[j]) < 0
	})

	return sorted
}

func suppressMatches(sorted []scan.Match, remaining map[baselineKey]int) ([]scan.Match, int) {
	regressions := make([]scan.Match, 0, len(sorted))
	suppressedCount := 0

	for _, match := range sorted {
		key := baselineKey{filePath: match.FilePath, message: match.Message}
		if remaining[key] > 0 {
			remaining[key]--
			suppressedCount++

			continue
		}

		regressions = append(regressions, match)
	}

	return regressions, suppressedCount
}

func countImprovements(remaining map[baselineKey]int) int {
	improvementsCount := 0
	for _, count := range remaining {
		if count > 0 {
			improvementsCount += count
		}
	}

	return improvementsCount
}

func compareMatch(left scan.Match, right scan.Match) int {
	if compared := cmp.Compare(left.FilePath, right.FilePath); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.Line, right.Line); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.Column, right.Column); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(severityRank(left.Severity), severityRank(right.Severity)); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.Message, right.Message); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.Root, right.Root); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.MatchText, right.MatchText); compared != 0 {
		return compared
	}

	return cmp.Compare(left.RuleIndex, right.RuleIndex)
}

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
