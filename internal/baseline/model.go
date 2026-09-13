// Package baseline loads and validates baseline JSON documents.
package baseline

import "github.com/reglint/reglint/internal/scan"

// Entry stores a count for one (filePath, message) key.
type Entry struct {
	FilePath string `json:"filePath"`
	Message  string `json:"message"`
	Count    int    `json:"count"`
}

// Document is the on-disk baseline JSON payload.
type Document struct {
	SchemaVersion int     `json:"schemaVersion"`
	Entries       []Entry `json:"entries"`
}

// Comparison is a deterministic baseline compare result.
type Comparison struct {
	Regressions       []scan.Match
	SuppressedCount   int
	ImprovementsCount int
}

// Generation is a deterministic baseline generation result.
type Generation struct {
	Document   Document
	EntryCount int
}
