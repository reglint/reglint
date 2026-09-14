package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/reglint/reglint/internal/scan"
)

const baselineFileMode = 0o600

type entryKey struct {
	filePath string
	message  string
}

// Generate builds a canonical baseline document from full scan matches.
func Generate(matches []scan.Match) Generation {
	counts := make(map[entryKey]int, len(matches))
	for _, match := range matches {
		key := entryKey{filePath: match.FilePath, message: match.Message}
		counts[key]++
	}

	entries := make([]Entry, 0, len(counts))
	for key, count := range counts {
		entries = append(entries, Entry{FilePath: key.filePath, Message: key.message, Count: count})
	}
	sortEntries(entries)

	return Generation{
		Document: Document{
			SchemaVersion: baselineSchemaVersion,
			Entries:       entries,
		},
		EntryCount: len(entries),
	}
}

// Write writes a canonical baseline JSON document to disk.
func Write(path string, document Document) error {
	canonical := canonicalizeDocument(document)

	payload, err := json.MarshalIndent(canonical, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, baselineFileMode); err != nil {
		return fmt.Errorf("write baseline: %w", err)
	}

	return nil
}

func canonicalizeDocument(document Document) Document {
	entries := append([]Entry{}, document.Entries...)
	sortEntries(entries)

	return Document{
		SchemaVersion: baselineSchemaVersion,
		Entries:       entries,
	}
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].FilePath != entries[j].FilePath {
			return entries[i].FilePath < entries[j].FilePath
		}

		return entries[i].Message < entries[j].Message
	})
}
