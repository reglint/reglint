//nolint:testpackage
package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reglint/reglint/internal/rules"
)

func TestCollectFilesFiltersByIncludeExclude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"))
	writeFile(t, filepath.Join(root, "src", "vendor", "skip.go"))
	writeFile(t, filepath.Join(root, "docs", "notes.md"))
	writeFile(t, filepath.Join(root, "README.md"))

	files, skipped, err := collectFiles(
		[]string{root},
		[]string{"src/**", "docs/**"},
		[]string{"**/vendor/**"},
		nil,
		1024,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped files, got %d", skipped)
	}

	want := []string{"docs/notes.md", "src/main.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("expected files %v, got %v", want, files)
	}
}

func TestMatchesPathHonorsExclude(t *testing.T) {
	t.Parallel()

	matched, err := matchesPath("src/vendor/skip.go", []string{"src/**"}, []string{"**/vendor/**"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Fatal("expected excluded path to be skipped")
	}
}

func TestMatchesPathRequiresInclude(t *testing.T) {
	t.Parallel()

	_, err := matchesPath("src/main.go", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty include patterns")
	}
}

func TestMatchesPathRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	_, err := matchesPath("src/main.go", []string{"["}, nil)
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestNormalizePatternsTrimsAndDropsEmpty(t *testing.T) {
	t.Parallel()

	got := normalizePatterns([]string{" src/** ", "", "  ", "docs/**"})
	want := []string{"src/**", "docs/**"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected patterns %v, got %v", want, got)
	}
}

func TestCollectFilesSkipsLargeFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "src", "small.txt"), "data")
	writeFileWithContent(t, filepath.Join(root, "src", "large.txt"), "0123456789")

	files, skipped, err := collectFiles([]string{root}, []string{"src/**"}, nil, nil, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped file, got %d", skipped)
	}

	want := []string{"src/small.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("expected files %v, got %v", want, files)
	}
}

func TestCollectFilesSkipsBinaryFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "src", "text.txt"), "hello")
	writeFileBytes(t, filepath.Join(root, "src", "binary.bin"), []byte{0x00, 0x01, 0x02})

	files, skipped, err := collectFiles([]string{root}, []string{"src/**"}, nil, nil, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped file, got %d", skipped)
	}

	want := []string{"src/text.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("expected files %v, got %v", want, files)
	}
}

func TestRunUsesConcurrencyForFileReads(t *testing.T) {
	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "a.txt"), "secret")
	writeFileWithContent(t, filepath.Join(root, "b.txt"), "secret")

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "secret",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include:          []string{"**/*"},
		Exclude:          nil,
		MaxFileSizeBytes: 1024,
		Concurrency:      2,
	}

	result, maxActive, runErr := runWithReadBlocking(t, request, 2)

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if maxActive < 2 {
		t.Fatalf("expected concurrent reads, max=%d", maxActive)
	}
	if len(result.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Matches))
	}
}

func updateMaxActive(maxActive *int64, current int64) {
	for {
		prev := atomic.LoadInt64(maxActive)
		if current <= prev || atomic.CompareAndSwapInt64(maxActive, prev, current) {
			return
		}
	}
}

func runWithReadBlocking(t *testing.T, request Request, expected int) (Result, int64, error) {
	t.Helper()
	originalReadFile := readFile
	defer func() {
		readFile = originalReadFile
	}()

	started := make(chan struct{}, expected)
	release := make(chan struct{})
	var active int64
	var maxActive int64
	readFile = func(path string) ([]byte, error) {
		current := atomic.AddInt64(&active, 1)
		updateMaxActive(&maxActive, current)
		started <- struct{}{}
		<-release
		data, err := originalReadFile(path)
		atomic.AddInt64(&active, -1)

		return data, err
	}

	done := make(chan struct{})
	var result Result
	var runErr error
	go func() {
		result, runErr = Run(request)
		close(done)
	}()

	waitForSignals(t, started, expected, "concurrent reads")
	close(release)
	awaitCompletion(t, done, "scan completion")

	return result, maxActive, runErr
}

func waitForSignals(t *testing.T, signals <-chan struct{}, count int, label string) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-signals:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for %s", label)
		}
	}
}

func awaitCompletion(t *testing.T, done <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", label)
	}
}

func TestRunCapturesRuneLineAndColumn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := "first line\nsecond ✓line\n"
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), content)

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "✓line",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include:          []string{"**/*"},
		Exclude:          nil,
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := Run(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}

	match := result.Matches[0]
	if match.FilePath != "sample.txt" {
		t.Fatalf("unexpected file path: %s", match.FilePath)
	}
	if match.Line != 2 || match.Column != 8 {
		t.Fatalf("unexpected position: %d:%d", match.Line, match.Column)
	}
	if match.MatchText != "✓line" {
		t.Fatalf("unexpected match text: %s", match.MatchText)
	}
	if match.Message != "Found ✓line" {
		t.Fatalf("unexpected match message: %s", match.Message)
	}
}

func TestRunFiltersMatchesByAddedLinesWhenEnabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "token=aaa\ntoken=bbb\n")

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "token=[a-z]+",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include: []string{"**/*"},
		Git: &GitSelectionRequest{
			Mode:           "staged",
			CandidateFiles: []string{"sample.txt"},
			AddedLinesOnly: true,
			AddedLinesByFile: map[string]map[int]struct{}{
				"sample.txt": {2: {}},
			},
		},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := Run(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match after added-lines filtering, got %d", len(result.Matches))
	}
	if result.Matches[0].Line != 2 {
		t.Fatalf("expected remaining match on line 2, got %d", result.Matches[0].Line)
	}
}

func TestRunAddedLinesOnlyDropsMatchesForFilesWithoutAddedLines(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "token=aaa\n")

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "token=[a-z]+",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include: []string{"**/*"},
		Git: &GitSelectionRequest{
			Mode:             "diff",
			CandidateFiles:   []string{"sample.txt"},
			AddedLinesOnly:   true,
			AddedLinesByFile: map[string]map[int]struct{}{},
		},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := Run(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("expected 0 matches when file has no added lines, got %d", len(result.Matches))
	}
	if result.Stats.Matches != 0 {
		t.Fatalf("expected stats.matches to be 0, got %d", result.Stats.Matches)
	}
	if result.Stats.FilesScanned != 1 {
		t.Fatalf("expected filesScanned to remain 1, got %d", result.Stats.FilesScanned)
	}
}

func TestRunAddedLinesOnlyIgnoredWhenGitModeOff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "token=aaa\n")

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "token=[a-z]+",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include: []string{"**/*"},
		Git: &GitSelectionRequest{
			Mode:           "off",
			CandidateFiles: []string{"sample.txt"},
			AddedLinesOnly: true,
			AddedLinesByFile: map[string]map[int]struct{}{
				"sample.txt": {},
			},
		},
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := Run(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected added-lines filter to be ignored in mode=off, got %d matches", len(result.Matches))
	}
}

func TestRunRejectsEmptyRegex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "content")

	request := Request{
		Roots: []string{root},
		Rules: []rules.Rule{
			{
				Message:  "Missing regex",
				Regex:    "",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include:          []string{"**/*"},
		Exclude:          nil,
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	_, err := Run(request)
	if err == nil {
		t.Fatal("expected error for empty regex")
	}
}

func TestRunRejectsEmptyInclude(t *testing.T) {
	t.Parallel()

	request := Request{
		Roots:            []string{"."},
		Rules:            nil,
		Include:          nil,
		Exclude:          nil,
		MaxFileSizeBytes: 1,
		Concurrency:      1,
	}

	_, err := Run(request)
	if err == nil {
		t.Fatal("expected error for empty include patterns")
	}
}

func TestRunScansFileRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	writeFileWithContent(t, path, "secret")

	request := Request{
		Roots: []string{path},
		Rules: []rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "secret",
				Severity: "warning",
				Paths:    []string{"**/*"},
			},
		},
		Include:          []string{"**/*"},
		Exclude:          nil,
		MaxFileSizeBytes: 1024,
		Concurrency:      1,
	}

	result, err := Run(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].FilePath != "sample.txt" {
		t.Fatalf("unexpected file path: %s", result.Matches[0].FilePath)
	}
}

func TestBuildCapturesHandlesMissingGroups(t *testing.T) {
	t.Parallel()

	content := "alpha"
	index := []int{0, 5, -1, -1}

	captures := buildCaptures(content, index)
	if len(captures) != 2 {
		t.Fatalf("expected 2 captures, got %d", len(captures))
	}
	if captures[0] != "alpha" {
		t.Fatalf("unexpected capture[0]: %s", captures[0])
	}
	if captures[1] != "" {
		t.Fatalf("unexpected capture[1]: %s", captures[1])
	}
}

func TestLineColumnFromIndexTracksRunes(t *testing.T) {
	t.Parallel()

	content := "one\nsecond ✓line"
	line, column := lineColumnFromIndex(content, len("one\nsecond "))
	if line != 2 || column != 8 {
		t.Fatalf("unexpected position: %d:%d", line, column)
	}
}

func TestSortMatchesOrdersBySeverityAndMessage(t *testing.T) {
	t.Parallel()

	matches := []Match{
		{
			Message:  "Zulu",
			Severity: "warning",
			FilePath: "a/file.go",
			Line:     2,
			Column:   3,
		},
		{
			Message:  "Alpha",
			Severity: "warning",
			FilePath: "a/file.go",
			Line:     2,
			Column:   3,
		},
		{
			Message:  "Error",
			Severity: "error",
			FilePath: "a/file.go",
			Line:     2,
			Column:   3,
		},
	}

	sortMatches(matches)

	if matches[0].Severity != "error" {
		t.Fatalf("expected error severity first, got %s", matches[0].Severity)
	}
	if matches[1].Message != "Alpha" || matches[2].Message != "Zulu" {
		t.Fatalf("unexpected message ordering: %s, %s", matches[1].Message, matches[2].Message)
	}
}

func TestSortMatchesOrdersByRootForSameFilePath(t *testing.T) {
	t.Parallel()

	matches := []Match{
		{
			Message:  "Same",
			Severity: "warning",
			FilePath: "same.txt",
			Root:     "root-b",
			Line:     1,
			Column:   1,
		},
		{
			Message:  "Same",
			Severity: "warning",
			FilePath: "same.txt",
			Root:     "root-a",
			Line:     1,
			Column:   1,
		},
	}

	sortMatches(matches)

	if matches[0].Root != "root-a" {
		t.Fatalf("expected root ordering, got %s", matches[0].Root)
	}
}

func TestSeverityRankDefaultsToUnknown(t *testing.T) {
	t.Parallel()

	if severityRank("custom") != severityRankUnknown {
		t.Fatalf("expected unknown severity rank")
	}
}

func TestScanEntriesCountsSkippedOnReadError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	entries := []fileEntry{{root: root, relPath: "missing.txt"}}
	rulesList := []rules.Rule{
		{
			Message:  "Found $0",
			Regex:    "content",
			Severity: "warning",
			Paths:    []string{"**/*"},
		},
	}

	compiled, err := compileRules(rulesList, []string{"**/*"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, filesScanned, filesSkipped, err := scanEntries(entries, compiled, 1, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filesSkipped != 1 {
		t.Fatalf("expected 1 skipped file, got %d", filesSkipped)
	}
	if filesScanned != 0 {
		t.Fatalf("expected 0 scanned files, got %d", filesScanned)
	}
}

func TestCollectEntriesKeepsRootOrderForSameRelPath(t *testing.T) {
	t.Parallel()

	rootA := t.TempDir()
	rootB := t.TempDir()
	writeFileWithContent(t, filepath.Join(rootA, "same.txt"), "alpha")
	writeFileWithContent(t, filepath.Join(rootB, "same.txt"), "beta")

	entries, skipped, err := collectEntries(
		[]string{rootB, rootA},
		[]string{"**/*"},
		nil,
		nil,
		1024,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped files, got %d", skipped)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].root != rootA {
		t.Fatalf("expected root order by path, got %s", entries[0].root)
	}
}

func TestCollectEntriesErrorsOnInvalidInclude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "data")

	_, _, err := collectEntries([]string{root}, []string{"["}, nil, nil, 1024)
	if err == nil {
		t.Fatal("expected error for invalid include pattern")
	}
}

func TestCollectFileEntryReturnsDirMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	entries, skipped, isFile, err := collectFileEntry(root, []string{"**/*"}, nil, nil, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isFile {
		t.Fatal("expected directory root to not be treated as file")
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestCollectFileEntrySkipsNonMatchingInclude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	writeFileWithContent(t, path, "content")

	entries, skipped, isFile, err := collectFileEntry(path, []string{"src/**"}, nil, nil, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isFile {
		t.Fatal("expected file root to be treated as file")
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestEvaluateFileSkipsOnUnreadableFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	writeFileWithContent(t, path, "content")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	entry := fs.FileInfoToDirEntry(info)
	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}

	selected, skipped, err := evaluateFile(path, "sample.txt", entry, []string{"**/*"}, nil, nil, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected {
		t.Fatal("expected file to be unselected")
	}
	if !skipped {
		t.Fatal("expected file to be skipped")
	}
}

func TestSeverityRankHandlesKnownValues(t *testing.T) {
	t.Parallel()

	values := []string{"error", "warning", "notice", "info"}
	for _, value := range values {
		if severityRank(value) == severityRankUnknown {
			t.Fatalf("expected severity %s to have known rank", value)
		}
	}
}

func TestCompileRulesUsesDefaultsAndRejectsInvalidRegex(t *testing.T) {
	t.Parallel()

	_, err := compileRules(
		[]rules.Rule{{Message: "bad", Regex: "[", Severity: "warning"}},
		[]string{"**/*.go"},
		[]string{"**/vendor/**"},
	)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}

	compiled, err := compileRules(
		[]rules.Rule{{Message: "ok", Regex: "abc", Severity: "warning"}},
		[]string{"**/*.go"},
		[]string{"**/vendor/**"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(compiled) != 1 {
		t.Fatalf("expected 1 compiled rule, got %d", len(compiled))
	}
	if len(compiled[0].paths) != 1 || compiled[0].paths[0] != "**/*.go" {
		t.Fatalf("unexpected default paths: %v", compiled[0].paths)
	}
	if len(compiled[0].exclude) != 1 || compiled[0].exclude[0] != "**/vendor/**" {
		t.Fatalf("unexpected default exclude: %v", compiled[0].exclude)
	}
}

func TestScanEntriesSkipsRuleWhenPathExcluded(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	writeFileWithContent(t, path, "secret")

	compiled, err := compileRules(
		[]rules.Rule{
			{
				Message:  "Found $0",
				Regex:    "secret",
				Severity: "warning",
				Paths:    []string{"**/*"},
				Exclude:  []string{"**/*.txt"},
			},
		},
		[]string{"**/*"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := []fileEntry{{root: root, relPath: "sample.txt"}}
	result, _, _, err := scanEntries(entries, compiled, 1, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected no matches, got %d", len(result))
	}
}

func TestScanEntriesErrorsOnInvalidRulePattern(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "sample.txt"), "abc")
	entries := []fileEntry{{root: root, relPath: "sample.txt"}}
	compiled := []compiledRule{{
		rule:    rules.Rule{Message: "Found $0", Regex: "abc", Severity: "warning"},
		regex:   regexp.MustCompile("abc"),
		paths:   []string{"["},
		exclude: nil,
	}}

	_, filesScanned, filesSkipped, err := scanEntries(entries, compiled, 1, false, nil)
	if err == nil {
		t.Fatal("expected error for invalid rule path pattern")
	}
	_ = filesScanned
	_ = filesSkipped
}

func writeFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	writeFileWithContent(t, path, "data")
}

func writeFileWithContent(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func writeFileBytes(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}
