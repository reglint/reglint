//nolint:testpackage
package scan

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectEntriesAppliesIgnoreRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "skip.txt\n")
	writeFileWithContent(t, filepath.Join(root, "keep.txt"), "keep")
	writeFileWithContent(t, filepath.Join(root, "skip.txt"), "skip")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, skipped, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped file, got %d", skipped)
	}

	paths := entryPaths(entries)
	if !reflect.DeepEqual(paths, []string{"keep.txt"}) {
		t.Fatalf("expected keep.txt only, got %v", paths)
	}
}

func TestCollectEntriesCountsIgnoredFilesAsSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "skip.txt\n")
	writeFileWithContent(t, filepath.Join(root, "skip.txt"), "skip")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, skipped, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped file, got %d", skipped)
	}
}

func TestCollectEntriesAppliesGitCandidateScopeBeforeIgnoreRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "noncandidate.txt\n")
	writeFileWithContent(t, filepath.Join(root, "candidate.txt"), "keep")
	writeFileWithContent(t, filepath.Join(root, "noncandidate.txt"), "skip")

	request := Request{
		Roots:   []string{root},
		Include: []string{"**/*.txt"},
		Exclude: nil,
		Ignore:  IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		Git: &GitSelectionRequest{
			Mode:           "staged",
			CandidateFiles: []string{"candidate.txt"},
		},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, skipped, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped files, got %d", skipped)
	}

	paths := entryPaths(entries)
	if !reflect.DeepEqual(paths, []string{"candidate.txt"}) {
		t.Fatalf("expected candidate.txt only, got %v", paths)
	}
}

func TestCollectEntriesGitCandidateScopeEmptyReturnsNoEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "candidate.txt"), "keep")

	request := Request{
		Roots:   []string{root},
		Include: []string{"**/*.txt"},
		Exclude: nil,
		Ignore:  IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		Git: &GitSelectionRequest{
			Mode:           "staged",
			CandidateFiles: []string{},
		},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, skipped, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped files, got %d", skipped)
	}
}

func TestCollectEntriesAllowsIgnoreNegation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "generated/**\n!generated/keep.txt\n")
	writeFileWithContent(t, filepath.Join(root, "generated", "keep.txt"), "keep")
	writeFileWithContent(t, filepath.Join(root, "generated", "skip.txt"), "skip")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, _, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	paths := entryPaths(entries)
	if !reflect.DeepEqual(paths, []string{"generated/keep.txt"}) {
		t.Fatalf("expected keep.txt only, got %v", paths)
	}
}

func TestCollectEntriesAllowsNestedReglintIgnoreNegation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "generated/**\n")
	writeFileWithContent(t, filepath.Join(root, "generated", ".reglintignore"), "!keep.txt\n")
	writeFileWithContent(t, filepath.Join(root, "generated", "keep.txt"), "keep")
	writeFileWithContent(t, filepath.Join(root, "generated", "skip.txt"), "skip")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore", ".reglintignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, _, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	paths := entryPaths(entries)
	if !reflect.DeepEqual(paths, []string{"generated/keep.txt"}) {
		t.Fatalf("expected keep.txt only, got %v", paths)
	}
}

func TestCollectEntriesNoIgnoreFilesScansIgnoredPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "skip.txt\n")
	writeFileWithContent(t, filepath.Join(root, "skip.txt"), "skip")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: false, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, skipped, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped files, got %d", skipped)
	}

	paths := entryPaths(entries)
	if !reflect.DeepEqual(paths, []string{"skip.txt"}) {
		t.Fatalf("expected skip.txt only, got %v", paths)
	}
}

func TestCollectEntriesReturnsIgnoreLoadErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "[\n")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	_, _, err := collectEntriesForTest(request)
	if err == nil {
		t.Fatal("expected error for invalid ignore pattern")
	}
}

func TestCollectEntriesIgnoreNegationDoesNotOverrideExclude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, ".ignore"), "!generated/keep.txt\n")
	writeFileWithContent(t, filepath.Join(root, "generated", "keep.txt"), "keep")

	request := Request{
		Roots:            []string{root},
		Include:          []string{"**/*.txt"},
		Exclude:          []string{"generated/**"},
		Ignore:           IgnoreSettings{Enabled: true, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	entries, _, err := collectEntriesForTest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestLoadIgnoreRulesDisabledReturnsNil(t *testing.T) {
	t.Parallel()

	request := Request{
		Roots:            []string{t.TempDir()},
		Include:          []string{"**/*"},
		Exclude:          nil,
		Ignore:           IgnoreSettings{Enabled: false, Files: []string{".ignore"}},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	rulesByRoot, err := loadIgnoreRules(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rulesByRoot != nil {
		t.Fatal("expected nil rules when ignore disabled")
	}
}

func entryPaths(entries []fileEntry) []string {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.relPath
	}

	return paths
}

func collectEntriesForTest(request Request) ([]fileEntry, int, error) {
	entries, skipped, _, include, exclude, err := collectScanEntries(request)
	_ = include
	_ = exclude

	return entries, skipped, err
}
