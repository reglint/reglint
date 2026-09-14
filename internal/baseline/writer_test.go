//nolint:testpackage // Validate internal behavior directly.
package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/scan"
)

func TestGenerateBuildsCanonicalBaselineDocument(t *testing.T) {
	t.Parallel()

	matches := []scan.Match{
		{FilePath: "src/b.go", Message: "m2"},
		{FilePath: "src/a.go", Message: "m2"},
		{FilePath: "src/a.go", Message: "m1"},
		{FilePath: "src/a.go", Message: "m1"},
	}

	generation := Generate(matches)

	if generation.Document.SchemaVersion != baselineSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", baselineSchemaVersion, generation.Document.SchemaVersion)
	}
	if generation.EntryCount != 3 {
		t.Fatalf("expected 3 unique entries, got %d", generation.EntryCount)
	}

	wantEntries := []Entry{
		{FilePath: "src/a.go", Message: "m1", Count: 2},
		{FilePath: "src/a.go", Message: "m2", Count: 1},
		{FilePath: "src/b.go", Message: "m2", Count: 1},
	}
	if !reflect.DeepEqual(generation.Document.Entries, wantEntries) {
		t.Fatalf("expected entries %+v, got %+v", wantEntries, generation.Document.Entries)
	}
}

func TestGenerateIsDeterministicForEquivalentInputs(t *testing.T) {
	t.Parallel()

	currentA := []scan.Match{
		{FilePath: "src/b.go", Message: "m2"},
		{FilePath: "src/a.go", Message: "m1"},
		{FilePath: "src/a.go", Message: "m1"},
	}
	currentB := []scan.Match{
		{FilePath: "src/a.go", Message: "m1"},
		{FilePath: "src/a.go", Message: "m1"},
		{FilePath: "src/b.go", Message: "m2"},
	}

	generationA := Generate(currentA)
	generationB := Generate(currentB)

	if !reflect.DeepEqual(generationA, generationB) {
		t.Fatalf("expected deterministic generation, got %+v and %+v", generationA, generationB)
	}
}

func TestWriteWritesCanonicalJSONAndOverwritesExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte("old-content"), 0o600); err != nil {
		t.Fatalf("failed to create baseline file: %v", err)
	}

	document := Document{
		SchemaVersion: baselineSchemaVersion,
		Entries: []Entry{
			{FilePath: "src/b.go", Message: "m2", Count: 1},
			{FilePath: "src/a.go", Message: "m2", Count: 1},
			{FilePath: "src/a.go", Message: "m1", Count: 2},
		},
	}

	if err := Write(path, document); err != nil {
		t.Fatalf("expected no error writing baseline, got %v", err)
	}

	decoded := decodeBaselineFile(t, path)
	if decoded.SchemaVersion != baselineSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", baselineSchemaVersion, decoded.SchemaVersion)
	}

	wantEntries := []Entry{
		{FilePath: "src/a.go", Message: "m1", Count: 2},
		{FilePath: "src/a.go", Message: "m2", Count: 1},
		{FilePath: "src/b.go", Message: "m2", Count: 1},
	}
	if !reflect.DeepEqual(decoded.Entries, wantEntries) {
		t.Fatalf("expected entries %+v, got %+v", wantEntries, decoded.Entries)
	}

	if err := Write(path, Document{
		SchemaVersion: baselineSchemaVersion,
		Entries:       []Entry{{FilePath: "src/c.go", Message: "m3", Count: 1}},
	}); err != nil {
		t.Fatalf("expected overwrite to succeed, got %v", err)
	}

	decoded = decodeBaselineFile(t, path)
	if len(decoded.Entries) != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", len(decoded.Entries))
	}
	if decoded.Entries[0].FilePath != "src/c.go" || decoded.Entries[0].Message != "m3" || decoded.Entries[0].Count != 1 {
		t.Fatalf("unexpected overwritten entry: %+v", decoded.Entries[0])
	}
}

func TestWriteReturnsErrorForUnwritableTarget(t *testing.T) {
	t.Parallel()

	err := Write(t.TempDir(), Document{SchemaVersion: baselineSchemaVersion})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "write baseline") {
		t.Fatalf("expected write baseline error, got %v", err)
	}
}

func decodeBaselineFile(t *testing.T, path string) Document {
	t.Helper()

	data, err := os.ReadFile(path) //#nosec G304 -- test controls temporary path
	if err != nil {
		t.Fatalf("failed to read baseline file: %v", err)
	}

	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("failed to decode baseline file: %v", err)
	}

	return document
}
