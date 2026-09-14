package scan

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/reglint/reglint/internal/ignore"
	"github.com/reglint/reglint/internal/rules"
)

var readFile = os.ReadFile
var walkRootTree = filepath.Walk

const (
	binaryProbeSize = 8000
	nullByte        = 0x00
)

const (
	captureIndexPairSize         = 2
	windowsAbsolutePathMinLength = 3
)

func collectFiles(
	roots []string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	maxFileSizeBytes int64,
) ([]string, int, error) {
	return collectFilesWithCandidates(roots, include, exclude, matcher, nil, maxFileSizeBytes)
}

func collectFilesWithCandidates(
	roots []string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]string, int, error) {
	var files []string
	skipped := 0

	for _, root := range roots {
		rootFiles, rootSkipped, err := collectRootFiles(root, include, exclude, matcher, candidateSet, maxFileSizeBytes)
		if err != nil {
			return nil, 0, err
		}

		skipped += rootSkipped
		files = append(files, rootFiles...)
	}

	sort.Strings(files)

	return files, skipped, nil
}

func collectRootFiles(
	root string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]string, int, error) {
	root = filepath.Clean(root)
	files := make([]string, 0)
	skipped := 0

	if err := walkRootTree(root, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}
		if !candidateSelected(relPath, candidateSet) {
			return nil
		}

		selected, fileSkipped, err := evaluateFile(
			filePath,
			relPath,
			fs.FileInfoToDirEntry(info),
			include,
			exclude,
			matcher,
			maxFileSizeBytes,
		)
		if err != nil {
			return err
		}
		if fileSkipped {
			skipped++

			return nil
		}
		if selected {
			files = append(files, relPath)
		}

		return nil
	}); err != nil {
		return nil, 0, err
	}

	return files, skipped, nil
}

type compiledRule struct {
	rule    rules.Rule
	regex   *regexp.Regexp
	paths   []string
	exclude []string
}

type fileEntry struct {
	root    string
	relPath string
}

type scanEntryResult struct {
	entryIndex int
	filePath   string
	matches    []Match
	scanned    bool
	err        error
}

type scanEntryWork struct {
	index int
	entry fileEntry
}

// Run executes a scan request and returns the aggregated result.
func Run(request Request) (Result, error) {
	start := time.Now()

	entries, skipped, include, exclude, err := collectScanEntries(request)
	if err != nil {
		return Result{}, err
	}

	compiled, err := compileRules(request.Rules, include, exclude)
	if err != nil {
		return Result{}, err
	}

	addedLinesOnly, addedLinesByFile := resolveAddedLinesFilter(request.Git)

	matches, filesScanned, filesSkipped, err := scanEntries(
		entries,
		compiled,
		request.Concurrency,
		addedLinesOnly,
		addedLinesByFile,
	)
	if err != nil {
		return Result{}, err
	}
	filesSkipped += skipped

	sortMatches(matches)

	result := Result{
		Matches: matches,
		Stats: Stats{
			FilesScanned: filesScanned,
			FilesSkipped: filesSkipped,
			Matches:      len(matches),
			DurationMs:   time.Since(start).Milliseconds(),
		},
	}

	return result, nil
}

func collectScanEntries(request Request) ([]fileEntry, int, []string, []string, error) {
	include := normalizePatterns(request.Include)
	exclude := normalizePatterns(request.Exclude)
	if len(include) == 0 {
		return nil, 0, nil, nil, errors.New("include patterns required")
	}

	candidateSet, err := buildCandidateSet(request)
	if err != nil {
		return nil, 0, nil, nil, err
	}

	matcherByRoot, err := loadIgnoreRules(request)
	if err != nil {
		return nil, 0, nil, nil, err
	}

	entries, skipped, err := collectEntriesWithCandidates(
		request.Roots,
		include,
		exclude,
		matcherByRoot,
		candidateSet,
		request.MaxFileSizeBytes,
	)
	if err != nil {
		return nil, 0, nil, nil, err
	}

	return entries, skipped, include, exclude, nil
}

func buildCandidateSet(request Request) (map[string]struct{}, error) {
	if request.Git == nil {
		return nil, nil
	}
	mode := strings.TrimSpace(request.Git.Mode)
	if mode == "" || mode == "off" {
		return nil, nil
	}

	candidateSet := make(map[string]struct{}, len(request.Git.CandidateFiles))
	for _, rawCandidate := range request.Git.CandidateFiles {
		normalized, err := normalizeCandidatePath(rawCandidate)
		if err != nil {
			return nil, err
		}
		if normalized == "" {
			continue
		}

		candidateSet[normalized] = struct{}{}
	}

	return candidateSet, nil
}

func normalizeCandidatePath(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", nil
	}

	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = path.Clean(normalized)
	if normalized == "." || normalized == "" {
		return "", nil
	}
	if path.IsAbs(normalized) || isWindowsAbsolutePath(normalized) {
		return "", fmt.Errorf("invalid git candidate file path %q", value)
	}
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("invalid git candidate file path %q", value)
	}

	return normalized, nil
}

func isWindowsAbsolutePath(value string) bool {
	if len(value) < windowsAbsolutePathMinLength {
		return false
	}
	drive := value[0]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) {
		return false
	}

	return value[1] == ':' && value[2] == '/'
}

func candidateSelected(relPath string, candidateSet map[string]struct{}) bool {
	if candidateSet == nil {
		return true
	}

	_, ok := candidateSet[relPath]

	return ok
}
func resolveAddedLinesFilter(gitRequest *GitSelectionRequest) (bool, map[string]map[int]struct{}) {
	if gitRequest == nil || !gitRequest.AddedLinesOnly {
		return false, nil
	}

	mode := strings.TrimSpace(gitRequest.Mode)
	if mode != "staged" && mode != "diff" {
		return false, nil
	}

	return true, gitRequest.AddedLinesByFile
}

func scanEntries(
	entries []fileEntry,
	compiled []compiledRule,
	concurrency int,
	addedLinesOnly bool,
	addedLinesByFile map[string]map[int]struct{},
) ([]Match, int, int, error) {
	var matches []Match
	filesScanned := 0
	filesSkipped := 0

	concurrency = requestConcurrency(entries, compiled, concurrency)
	if concurrency == 1 {
		return scanEntriesSequential(entries, compiled, addedLinesOnly, addedLinesByFile)
	}

	entryCh := make(chan scanEntryWork)
	resultCh := make(chan scanEntryResult)
	workerCount := minInt(concurrency, len(entries))

	for i := 0; i < workerCount; i++ {
		go func() {
			for work := range entryCh {
				result := scanEntry(work.entry, compiled, work.index, addedLinesOnly, addedLinesByFile)
				resultCh <- result
			}
		}()
	}

	go func() {
		for index, entry := range entries {
			entryCh <- scanEntryWork{index: index, entry: entry}
		}
		close(entryCh)
	}()

	results := make([]scanEntryResult, len(entries))
	for i := 0; i < len(entries); i++ {
		result := <-resultCh
		results[result.entryIndex] = result
	}

	for _, result := range results {
		if result.err != nil {
			return nil, 0, 0, result.err
		}
		if !result.scanned {
			filesSkipped++

			continue
		}
		filesScanned++
		matches = append(matches, result.matches...)
	}

	return matches, filesScanned, filesSkipped, nil
}

func matcherForRoot(matcherByRoot map[string][]ignore.IgnoreRule, root string) []ignore.IgnoreRule {
	if matcherByRoot == nil {
		return nil
	}

	root = filepath.Clean(root)

	return matcherByRoot[root]
}

func scanEntriesSequential(
	entries []fileEntry,
	compiled []compiledRule,
	addedLinesOnly bool,
	addedLinesByFile map[string]map[int]struct{},
) ([]Match, int, int, error) {
	var matches []Match
	filesScanned := 0
	filesSkipped := 0

	for index, entry := range entries {
		result := scanEntry(entry, compiled, index, addedLinesOnly, addedLinesByFile)
		if result.err != nil {
			return nil, 0, 0, result.err
		}
		if !result.scanned {
			filesSkipped++

			continue
		}
		filesScanned++
		matches = append(matches, result.matches...)
	}

	return matches, filesScanned, filesSkipped, nil
}

func scanEntry(
	entry fileEntry,
	compiled []compiledRule,
	entryIndex int,
	addedLinesOnly bool,
	addedLinesByFile map[string]map[int]struct{},
) scanEntryResult {
	fullPath := filepath.Join(entry.root, filepath.FromSlash(entry.relPath))
	contentBytes, err := readFile(fullPath)
	if err != nil {
		return scanEntryResult{entryIndex: entryIndex, filePath: entry.relPath}
	}
	content := string(contentBytes)

	matches := make([]Match, 0)
	for _, rule := range compiled {
		match, err := matchesPath(entry.relPath, rule.paths, rule.exclude)
		if err != nil {
			return scanEntryResult{filePath: entry.relPath, err: err}
		}
		if !match {
			continue
		}

		indices := rule.regex.FindAllStringSubmatchIndex(content, -1)
		for _, index := range indices {
			captures := buildCaptures(content, index)
			line, column := lineColumnFromIndex(content, index[0])
			if !shouldKeepMatchByAddedLines(addedLinesOnly, addedLinesByFile, entry.relPath, line) {
				continue
			}
			message := rules.InterpolateMessage(rule.rule.Message, captures)

			matches = append(matches, Match{
				Message:   message,
				Severity:  rule.rule.Severity,
				FilePath:  entry.relPath,
				Root:      entry.root,
				Line:      line,
				Column:    column,
				MatchText: captures[0],
				RuleIndex: rule.rule.Index,
			})
		}
	}

	return scanEntryResult{
		entryIndex: entryIndex,
		filePath:   entry.relPath,
		matches:    matches,
		scanned:    true,
	}
}

func shouldKeepMatchByAddedLines(
	addedLinesOnly bool,
	addedLinesByFile map[string]map[int]struct{},
	filePath string,
	line int,
) bool {
	if !addedLinesOnly {
		return true
	}

	lineSet, ok := addedLinesByFile[filePath]
	if !ok {
		return false
	}

	_, keep := lineSet[line]

	return keep
}

func requestConcurrency(entries []fileEntry, compiled []compiledRule, defaultValue int) int {
	if len(entries) == 0 || len(compiled) == 0 {
		return 1
	}

	if defaultValue <= 0 {
		return 1
	}

	return defaultValue
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func collectEntries(
	roots []string,
	include []string,
	exclude []string,
	matcherByRoot map[string][]ignore.IgnoreRule,
	maxFileSizeBytes int64,
) ([]fileEntry, int, error) {
	return collectEntriesWithCandidates(roots, include, exclude, matcherByRoot, nil, maxFileSizeBytes)
}

func collectEntriesWithCandidates(
	roots []string,
	include []string,
	exclude []string,
	matcherByRoot map[string][]ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]fileEntry, int, error) {
	entries := make([]fileEntry, 0)
	skipped := 0

	for _, root := range roots {
		matcher := matcherForRoot(matcherByRoot, root)
		fileEntries, fileSkipped, isFile, err := collectFileEntryWithCandidates(
			root,
			include,
			exclude,
			matcher,
			candidateSet,
			maxFileSizeBytes,
		)
		if err != nil {
			return nil, 0, err
		}
		if isFile {
			skipped += fileSkipped
			entries = append(entries, fileEntries...)

			continue
		}

		files, fileSkipped, err := collectFilesWithCandidates(
			[]string{root},
			include,
			exclude,
			matcher,
			candidateSet,
			maxFileSizeBytes,
		)
		if err != nil {
			return nil, 0, err
		}
		skipped += fileSkipped
		for _, relPath := range files {
			entries = append(entries, fileEntry{root: root, relPath: relPath})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].relPath != entries[j].relPath {
			return entries[i].relPath < entries[j].relPath
		}

		return entries[i].root < entries[j].root
	})

	return entries, skipped, nil
}

func collectFileEntry(
	root string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	maxFileSizeBytes int64,
) ([]fileEntry, int, bool, error) {
	return collectFileEntryWithCandidates(root, include, exclude, matcher, nil, maxFileSizeBytes)
}

func collectFileEntryWithCandidates(
	root string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]fileEntry, int, bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, 0, false, err
	}
	if info.IsDir() {
		return nil, 0, false, nil
	}

	rootDir := filepath.Dir(root)
	relPath, err := filepath.Rel(rootDir, root)
	if err != nil {
		return nil, 0, true, err
	}
	relPath = filepath.ToSlash(relPath)
	if !candidateSelected(relPath, candidateSet) {
		return nil, 0, true, nil
	}
	entry := fs.FileInfoToDirEntry(info)
	selected, fileSkipped, err := evaluateFile(
		root,
		relPath,
		entry,
		include,
		exclude,
		matcher,
		maxFileSizeBytes,
	)
	if err != nil {
		return nil, 0, true, err
	}
	if fileSkipped {
		return nil, 1, true, nil
	}
	if selected {
		return []fileEntry{{root: rootDir, relPath: relPath}}, 0, true, nil
	}

	return nil, 0, true, nil
}

func compileRules(ruleList []rules.Rule, defaultInclude []string, defaultExclude []string) ([]compiledRule, error) {
	compiled := make([]compiledRule, len(ruleList))
	for i, rule := range ruleList {
		if strings.TrimSpace(rule.Regex) == "" {
			return nil, errors.New("rule regex must not be empty")
		}
		regex, err := regexp.Compile(rule.Regex)
		if err != nil {
			return nil, err
		}
		if rule.Index == 0 {
			rule.Index = i
		}
		paths := normalizePatterns(rule.Paths)
		if len(paths) == 0 {
			paths = append([]string{}, defaultInclude...)
		}
		exclude := normalizePatterns(rule.Exclude)
		if len(exclude) == 0 {
			exclude = append([]string{}, defaultExclude...)
		}
		compiled[i] = compiledRule{
			rule:    rule,
			regex:   regex,
			paths:   paths,
			exclude: exclude,
		}
	}

	return compiled, nil
}

func buildCaptures(content string, index []int) []string {
	count := len(index) / captureIndexPairSize
	captures := make([]string, count)
	for i := 0; i < count; i++ {
		start := index[i*2]
		end := index[i*2+1]
		if start == -1 || end == -1 {
			captures[i] = ""

			continue
		}
		captures[i] = content[start:end]
	}

	return captures
}

func lineColumnFromIndex(content string, byteIndex int) (int, int) {
	line := 1
	column := 1
	for idx, runeValue := range content {
		if idx >= byteIndex {
			break
		}
		if runeValue == '\n' {
			line++
			column = 1

			continue
		}
		column++
	}

	return line, column
}

func sortMatches(matches []Match) {
	sort.Slice(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.FilePath != right.FilePath {
			return left.FilePath < right.FilePath
		}
		if left.Root != right.Root {
			return left.Root < right.Root
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
}

const (
	severityRankError = iota
	severityRankWarning
	severityRankNotice
	severityRankInfo
	severityRankUnknown
)

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

func matchesPath(path string, include []string, exclude []string) (bool, error) {
	if len(include) == 0 {
		return false, errors.New("include patterns required")
	}

	for _, pattern := range exclude {
		ok, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}

	for _, pattern := range include {
		ok, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func normalizePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func evaluateFile(
	path string,
	relPath string,
	entry os.DirEntry,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	maxFileSizeBytes int64,
) (bool, bool, error) {
	match, err := matchesPath(relPath, include, exclude)
	if err != nil {
		return false, false, err
	}
	if !match {
		return false, false, nil
	}
	if matcher != nil {
		ignored, err := ignore.Match(matcher, relPath, entry.IsDir())
		if err != nil {
			return false, false, err
		}
		if ignored {
			return false, true, nil
		}
	}

	skip, err := shouldSkipFile(path, entry, maxFileSizeBytes)
	if err != nil {
		return false, false, err
	}
	if skip {
		return false, true, nil
	}

	return true, false, nil
}

func shouldSkipFile(path string, entry os.DirEntry, maxFileSizeBytes int64) (bool, error) {
	info, err := entry.Info()
	if err != nil {
		return false, err
	}
	if maxFileSizeBytes > 0 && info.Size() > maxFileSizeBytes {
		return true, nil
	}

	file, err := os.Open(path) //#nosec G304 -- path is from filepath.Walk within controlled directory
	if err != nil {
		return true, nil
	}

	buffer := make([]byte, binaryProbeSize)
	read, readErr := file.Read(buffer)
	closeErr := file.Close()
	if closeErr != nil {
		return true, nil
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return true, nil
	}
	for _, value := range buffer[:read] {
		if value == nullByte {
			return true, nil
		}
	}

	return false, nil
}
