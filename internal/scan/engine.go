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
var walkRootTree = filepath.WalkDir

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
) ([]string, int, []string, error) {
	return collectFilesWithCandidates(roots, include, exclude, matcher, nil, maxFileSizeBytes)
}

func collectFilesWithCandidates(
	roots []string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]string, int, []string, error) {
	var files []string
	var warnings []string
	skipped := 0

	for _, root := range roots {
		rootFiles, rootSkipped, rootWarnings, err := collectRootFiles(
			root, include, exclude, matcher, candidateSet, maxFileSizeBytes,
		)
		if err != nil {
			return nil, 0, nil, err
		}

		skipped += rootSkipped
		files = append(files, rootFiles...)
		warnings = append(warnings, rootWarnings...)
	}

	sort.Strings(files)

	return files, skipped, warnings, nil
}

func collectRootFiles(
	root string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]string, int, []string, error) {
	root = filepath.Clean(root)

	if candidateSet != nil {
		return collectCandidateFiles(root, include, exclude, matcher, candidateSet, maxFileSizeBytes)
	}

	collector := &rootWalkCollector{
		root:             root,
		include:          include,
		exclude:          exclude,
		matcher:          matcher,
		candidateSet:     candidateSet,
		maxFileSizeBytes: maxFileSizeBytes,
		files:            make([]string, 0),
	}

	if err := walkRootTree(root, collector.collectEntry); err != nil {
		return nil, 0, nil, err
	}

	return collector.files, collector.skipped, collector.warnings, nil
}

// rootWalkCollector accumulates the results of one walked scan root.
type rootWalkCollector struct {
	root             string
	include          []string
	exclude          []string
	matcher          []ignore.IgnoreRule
	candidateSet     map[string]struct{}
	maxFileSizeBytes int64
	files            []string
	warnings         []string
	skipped          int
}

func (collector *rootWalkCollector) collectEntry(entryPath string, entry fs.DirEntry, err error) error {
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			collector.warnings = append(collector.warnings, fmt.Sprintf("skipping unreadable path: %s", entryPath))

			return nil
		}

		return err
	}
	if entry.IsDir() {
		pruned, pruneErr := walkDirPrune(collector.root, entryPath, collector.exclude)
		if pruneErr != nil {
			return pruneErr
		}
		if pruned {
			return filepath.SkipDir
		}

		return nil
	}

	relPath, selected, fileSkipped, fileErr := classifyWalkedFile(
		collector.root,
		entryPath,
		entry,
		collector.include,
		collector.exclude,
		collector.matcher,
		collector.candidateSet,
		collector.maxFileSizeBytes,
	)
	if fileErr != nil {
		return fileErr
	}
	if fileSkipped {
		collector.skipped++

		return nil
	}
	if selected {
		collector.files = append(collector.files, relPath)
	}

	return nil
}

// classifyWalkedFile resolves a walked non-directory entry to a scan candidate:
// its slash-normalized rel path, whether it is selected, and whether it counts
// as skipped (ignore rules, size or binary heuristics).
func classifyWalkedFile(
	root string,
	filePath string,
	entry fs.DirEntry,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) (string, bool, bool, error) {
	relPath, err := filepath.Rel(root, filePath)
	if err != nil {
		return "", false, false, err
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "." || !candidateSelected(relPath, candidateSet) {
		return relPath, false, false, nil
	}

	selected, fileSkipped, err := evaluateFile(
		filePath,
		relPath,
		entry,
		include,
		exclude,
		matcher,
		maxFileSizeBytes,
	)
	if err != nil {
		return "", false, false, err
	}

	return relPath, selected, fileSkipped, nil
}

// collectCandidateFiles scans only the git candidate set, never walking the
// tree, so non-candidate (possibly unreadable) directories are not touched.
func collectCandidateFiles(
	root string,
	include []string,
	exclude []string,
	matcher []ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]string, int, []string, error) {
	candidates := make([]string, 0, len(candidateSet))
	for rel := range candidateSet {
		candidates = append(candidates, rel)
	}
	sort.Strings(candidates)

	files := make([]string, 0)
	skipped := 0

	for _, rel := range candidates {
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil || info.IsDir() {
			continue
		}

		selected, fileSkipped, err := evaluateFile(
			full,
			rel,
			fs.FileInfoToDirEntry(info),
			include,
			exclude,
			matcher,
			maxFileSizeBytes,
		)
		if err != nil {
			return nil, 0, nil, err
		}
		if fileSkipped {
			skipped++

			continue
		}
		if selected {
			files = append(files, rel)
		}
	}

	return files, skipped, nil, nil
}

// walkDirPrune decides whether a walked directory must be skipped before its
// contents are read; pruning happens before ReadDir so excluded (and even
// unreadable) directories are never opened.
func walkDirPrune(root string, dirPath string, exclude []string) (bool, error) {
	relDir, err := filepath.Rel(root, dirPath)
	if err != nil {
		return false, err
	}
	relDir = filepath.ToSlash(relDir)
	if relDir == "." {
		return false, nil
	}

	return dirPruned(relDir, exclude)
}

func dirPruned(relDir string, exclude []string) (bool, error) {
	for _, pattern := range exclude {
		if pattern == relDir || strings.HasPrefix(pattern, relDir+"/") {
			return true, nil
		}
		matched, err := doublestar.Match(pattern, relDir)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
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

	entries, skipped, warnings, include, exclude, err := collectScanEntries(request)
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
		Matches:  matches,
		Warnings: warnings,
		Stats: Stats{
			FilesScanned: filesScanned,
			FilesSkipped: filesSkipped,
			Matches:      len(matches),
			DurationMs:   time.Since(start).Milliseconds(),
		},
	}

	return result, nil
}

func collectScanEntries(request Request) ([]fileEntry, int, []string, []string, []string, error) {
	include := normalizePatterns(request.Include)
	exclude := normalizePatterns(request.Exclude)
	if len(include) == 0 {
		return nil, 0, nil, nil, nil, errors.New("include patterns required")
	}

	candidateSet, err := buildCandidateSet(request)
	if err != nil {
		return nil, 0, nil, nil, nil, err
	}

	matcherByRoot, err := loadIgnoreRules(request)
	if err != nil {
		return nil, 0, nil, nil, nil, err
	}

	entries, skipped, warnings, err := collectEntriesWithCandidates(
		request.Roots,
		include,
		exclude,
		matcherByRoot,
		candidateSet,
		request.MaxFileSizeBytes,
	)
	if err != nil {
		return nil, 0, nil, nil, nil, err
	}

	return entries, skipped, warnings, include, exclude, nil
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
) ([]fileEntry, int, []string, error) {
	return collectEntriesWithCandidates(roots, include, exclude, matcherByRoot, nil, maxFileSizeBytes)
}

func collectEntriesWithCandidates(
	roots []string,
	include []string,
	exclude []string,
	matcherByRoot map[string][]ignore.IgnoreRule,
	candidateSet map[string]struct{},
	maxFileSizeBytes int64,
) ([]fileEntry, int, []string, error) {
	entries := make([]fileEntry, 0)
	var warnings []string
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
			return nil, 0, nil, err
		}
		if isFile {
			skipped += fileSkipped
			entries = append(entries, fileEntries...)

			continue
		}

		files, fileSkipped, fileWarnings, err := collectFilesWithCandidates(
			[]string{root},
			include,
			exclude,
			matcher,
			candidateSet,
			maxFileSizeBytes,
		)
		if err != nil {
			return nil, 0, nil, err
		}
		skipped += fileSkipped
		warnings = append(warnings, fileWarnings...)
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

	return entries, skipped, warnings, nil
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

// explicitFileBase normalizes explicit file arguments to cwd-relative paths so
// config/CLI include/exclude globs share the same base as the default walk
// (root "." from cwd). Falls back to parent dir + basename for paths outside
// the working directory.
func explicitFileBase(root string) (string, string) {
	if abs, err := filepath.Abs(root); err == nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			if rel, relErr := filepath.Rel(wd, abs); relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return wd, filepath.ToSlash(rel)
			}
		}
	}

	return filepath.Dir(root), filepath.ToSlash(filepath.Base(root))
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

	rootDir, relPath := explicitFileBase(root)
	// shortcut: ignore files don't apply to explicit-file roots; excludes/includes use cwd base
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
