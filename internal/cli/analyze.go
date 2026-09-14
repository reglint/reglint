package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/reglint/reglint/internal/baseline"
	"github.com/reglint/reglint/internal/config"
	"github.com/reglint/reglint/internal/git"
	"github.com/reglint/reglint/internal/hooks"
	"github.com/reglint/reglint/internal/output"
	"github.com/reglint/reglint/internal/rules"
	"github.com/reglint/reglint/internal/scan"
)

const (
	defaultConfigPath   = "reglint-rules.yaml"
	defaultFormat       = "console"
	defaultMaxFileBytes = int64(5242880)
)

const (
	exitCodeError   = 1
	exitCodeFailOn  = 2
	defaultFileMode = 0o600
)

// Config holds parsed analyze command inputs.
type Config struct {
	ConfigPath            string
	Roots                 []string
	Formats               []string
	OutJSON               string
	OutSARIF              string
	Include               []string
	Exclude               []string
	Concurrency           int
	ConcurrencySet        bool
	MaxFileSizeBytes      int64
	FailOnSeverity        string
	BaselinePath          string
	RuleSetBaselinePath   string
	EffectiveBaselinePath string
	WriteBaseline         bool
	GitMode               string
	GitModeSet            bool
	GitDiffTarget         string
	GitDiffSet            bool
	GitAddedLinesOnly     bool
	GitAddedLinesOnlySet  bool
	GitignoreEnabled      bool
	NoGitignore           bool
	NoIgnoreFiles         bool
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)

	return nil
}

// HandleAnalyze executes the analyze command.
func HandleAnalyze(args []string, out *bytes.Buffer) int {
	result, failOn, formats, ruleset, cfg, consoleColors, err := runAnalyze(args)
	if err != nil {
		writeError(out, err)

		return exitCodeError
	}

	if err := renderOutputs(formats, ruleset, cfg, consoleColors, result, out); err != nil {
		writeError(out, err)

		return exitCodeError
	}

	if failOn == "" {
		return 0
	}

	return exitCodeForFailOn(result.Matches, failOn)
}

// ParseAnalyzeArgs parses analyze command arguments into a Config.
func ParseAnalyzeArgs(args []string) (Config, error) {
	var cfg Config

	flagSet := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flagSet.SetOutput(&strings.Builder{})

	flagSet.StringVar(&cfg.ConfigPath, "config", defaultConfigPath, "Path to YAML rules config file.")
	flagSet.StringVar(&cfg.ConfigPath, "c", defaultConfigPath, "Path to YAML rules config file.")
	formatValue := flagSet.String("format", defaultFormat, "Comma-separated list of formats.")
	formatShort := flagSet.String("f", defaultFormat, "Comma-separated list of formats.")
	flagSet.StringVar(&cfg.OutJSON, "out-json", "", "Output path for JSON results.")
	flagSet.StringVar(&cfg.OutSARIF, "out-sarif", "", "Output path for SARIF results.")
	var include stringSlice
	var exclude stringSlice
	flagSet.Var(&include, "include", "Repeatable include glob.")
	flagSet.Var(&exclude, "exclude", "Repeatable exclude glob.")
	flagSet.IntVar(&cfg.Concurrency, "concurrency", runtime.GOMAXPROCS(0), "Worker count.")
	flagSet.Int64Var(&cfg.MaxFileSizeBytes, "max-file-size", defaultMaxFileBytes, "Maximum file size in bytes.")
	flagSet.StringVar(&cfg.FailOnSeverity, "fail-on", "", "Fail if matches at or above severity.")
	flagSet.StringVar(&cfg.BaselinePath, "baseline", "", "Baseline JSON path for suppression.")
	flagSet.BoolVar(&cfg.WriteBaseline, "write-baseline", false, "Generate/regenerate baseline from findings.")
	flagSet.StringVar(&cfg.GitMode, "git-mode", "off", "Select Git mode: off, staged, or diff.")
	flagSet.StringVar(&cfg.GitDiffTarget, "git-diff", "", "Diff target/range for git mode diff.")
	flagSet.BoolVar(&cfg.GitAddedLinesOnly, "git-added-lines-only", false, "Restrict matches to added lines in Git mode.")
	flagSet.BoolVar(&cfg.NoGitignore, "no-gitignore", false, "Disable .gitignore filtering for this run.")
	flagSet.BoolVar(&cfg.NoIgnoreFiles, "no-ignore-files", false, "Disable ignore file loading and matching.")

	if err := flagSet.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Include = include
	cfg.Exclude = exclude
	cfg.GitMode = strings.TrimSpace(cfg.GitMode)
	cfg.GitDiffTarget = strings.TrimSpace(cfg.GitDiffTarget)
	applyAnalyzeFlagPresence(flagSet, &cfg)
	cfg.Roots = resolveAnalyzeRoots(flagSet.Args())

	formatInput := selectFormatInput(flagSet, *formatValue, *formatShort)
	formats, err := parseFormats(formatInput)
	if err != nil {
		return Config{}, err
	}
	cfg.Formats = formats

	if err := validateAnalyzeConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyAnalyzeFlagPresence(flagSet *flag.FlagSet, cfg *Config) {
	cfg.ConcurrencySet = wasFlagProvided(flagSet, "concurrency")
	cfg.GitModeSet = wasFlagProvided(flagSet, "git-mode")
	cfg.GitDiffSet = wasFlagProvided(flagSet, "git-diff")
	cfg.GitAddedLinesOnlySet = wasFlagProvided(flagSet, "git-added-lines-only")
	cfg.GitignoreEnabled = !cfg.NoGitignore
	if cfg.GitDiffSet {
		cfg.GitMode = "diff"
	}
}

func resolveAnalyzeRoots(args []string) []string {
	if len(args) == 0 {
		return []string{"."}
	}

	return append([]string{}, args...)
}

func selectFormatInput(flagSet *flag.FlagSet, longValue string, shortValue string) string {
	if wasFlagProvided(flagSet, "f") && !wasFlagProvided(flagSet, "format") {
		return shortValue
	}

	return longValue
}

func parseFormats(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("format must not be empty")
	}

	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		format := strings.TrimSpace(part)
		if format == "" {
			return nil, errors.New("format must not be empty")
		}
		if _, ok := seen[format]; ok {
			continue
		}
		seen[format] = struct{}{}
		result = append(result, format)
	}

	registry, err := output.NewRegistry(
		output.ConsoleFormatter{},
		output.JSONFormatter{},
		output.SARIFFormatter{},
	)
	if err != nil {
		return nil, err
	}
	if _, err := registry.Resolve(result); err != nil {
		return nil, err
	}

	return result, nil
}

func validateAnalyzeConfig(cfg Config) error {
	if err := validateConfigPath(cfg.ConfigPath); err != nil {
		return err
	}
	if cfg.Concurrency <= 0 {
		return errors.New("concurrency must be positive")
	}
	if cfg.MaxFileSizeBytes <= 0 {
		return errors.New("max-file-size must be positive")
	}
	if cfg.FailOnSeverity != "" && !isValidSeverity(cfg.FailOnSeverity) {
		return fmt.Errorf("invalid fail-on value: %s", cfg.FailOnSeverity)
	}
	if err := validateOutputPaths(cfg); err != nil {
		return err
	}
	if err := validateGitCLIInputs(cfg); err != nil {
		return err
	}

	return nil
}

func validateGitCLIInputs(cfg Config) error {
	if !isValidGitMode(cfg.GitMode) {
		return errors.New("--git-mode must be one of off, staged, diff")
	}
	if cfg.GitDiffSet && cfg.GitDiffTarget == "" {
		return errors.New("--git-diff must be a non-empty string")
	}

	return nil
}

func isValidGitMode(mode string) bool {
	switch mode {
	case "off", "staged", "diff":
		return true
	default:
		return false
	}
}

func validateConfigPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("config file not found: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("config path is a directory: %s", path)
	}
	file, err := os.Open(path) //#nosec G304 -- path comes from user input and is validated here
	if err != nil {
		return fmt.Errorf("config file not readable: %s", path)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("config file not readable: %s", path)
	}

	return nil
}

func validateOutputPaths(cfg Config) error {
	if len(cfg.Formats) <= 1 {
		return validateOutputPathValues(cfg)
	}

	for _, format := range cfg.Formats {
		switch format {
		case "json":
			if cfg.OutJSON == "" {
				return errors.New("--out-json is required when requesting json with multiple formats")
			}
		case "sarif":
			if cfg.OutSARIF == "" {
				return errors.New("--out-sarif is required when requesting sarif with multiple formats")
			}
		}
	}

	return validateOutputPathValues(cfg)
}

func validateOutputPathValues(cfg Config) error {
	if cfg.OutJSON != "" {
		if err := validateOutputPath(cfg.OutJSON); err != nil {
			return err
		}
	}
	if cfg.OutSARIF != "" {
		if err := validateOutputPath(cfg.OutSARIF); err != nil {
			return err
		}
	}

	return nil
}

func validateOutputPath(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		return validateExistingOutputPath(info, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("output path not writable: %s", path)
	}

	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		parent = "."
	}
	if _, err := os.Stat(parent); err != nil {
		return fmt.Errorf("output path not writable: %s", path)
	}

	return validateOutputDirectoryWritable(parent, path)
}

func validateExistingOutputPath(info os.FileInfo, path string) error {
	if info.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, defaultFileMode) //#nosec G304 -- validated output path
	if err != nil {
		return fmt.Errorf("output path not writable: %s", path)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("output path not writable: %s", path)
	}

	return nil
}

func validateOutputDirectoryWritable(parent, path string) error {
	probe, err := os.CreateTemp(parent, ".reglint-*")
	if err != nil {
		return fmt.Errorf("output path not writable: %s", path)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)

		return fmt.Errorf("output path not writable: %s", path)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("output path not writable: %s", path)
	}

	return nil
}

func isValidSeverity(value string) bool {
	switch value {
	case "error", "warning", "notice", "info":
		return true
	default:
		return false
	}
}

// BuildScanRequest resolves overrides and builds a scan request.
func BuildScanRequest(cfg Config, ruleSet config.RuleSet) (scan.Request, string, output.ConsoleColorSettings) {
	effective := ruleSet.ToRules()

	applyRuleSetOverrides(cfg, &effective)

	resolvedGit := effective.Git
	if settings, err := resolveEffectiveGitSettings(cfg, effective.Git); err == nil {
		resolvedGit = settings
	}

	ignoreSettings := resolveIgnoreSettings(effective, resolvedGit)
	consoleColorSettings := resolveConsoleColorSettings(effective)

	request := scan.Request{
		Roots:            append([]string{}, cfg.Roots...),
		Rules:            buildEffectiveRules(cfg, effective),
		Include:          append([]string{}, effective.Include...),
		Exclude:          append([]string{}, effective.Exclude...),
		Ignore:           ignoreSettings,
		Git:              buildGitRequest(resolvedGit),
		MaxFileSizeBytes: cfg.MaxFileSizeBytes,
		Concurrency:      resolveConcurrency(cfg, effective.Concurrency),
	}

	return request, resolveFailOn(effective.FailOn), consoleColorSettings
}

func writeError(out *bytes.Buffer, err error) {
	_, _ = fmt.Fprintf(out, "%s\n", err.Error())
}

func runAnalyze(
	args []string,
) (scan.Result, string, []string, []rules.Rule, Config, output.ConsoleColorSettings, error) {
	cfg, err := ParseAnalyzeArgs(args)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	ruleSet, err := config.LoadRuleSet(cfg.ConfigPath)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	cfg, err = prepareAnalyzeConfig(cfg, ruleSet)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	request, failOn, consoleColorSettings := BuildScanRequest(cfg, ruleSet)
	hookRegistry, hookContext := buildScanHooks(cfg, request)
	if err := hookRegistry.OnCapabilitiesCheck(hookContext); err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	scope, err := hookRegistry.BeforeCollectCandidates(hookContext)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}
	applyCandidateScope(&request, &hookContext, scope)

	ignoreAugmentation, err := hookRegistry.BeforeIgnoreEvaluation(hookContext)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}
	applyIgnoreAugmentation(&request, ignoreAugmentation)

	result, err := scan.Run(request)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	result, err = applyAfterMatchHooks(result, hookRegistry, hookContext)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	result, failOn, err = applyBaselineMode(cfg, result, failOn)
	if err != nil {
		return scan.Result{}, "", nil, nil, Config{}, output.ConsoleColorSettings{}, err
	}

	return result, failOn, cfg.Formats, request.Rules, cfg, consoleColorSettings, nil
}

func prepareAnalyzeConfig(cfg Config, ruleSet config.RuleSet) (Config, error) {
	cliPath, ruleSetPath, effectivePath, err := resolveBaselinePaths(cfg, ruleSet)
	if err != nil {
		return Config{}, err
	}

	cfg.BaselinePath = cliPath
	cfg.RuleSetBaselinePath = ruleSetPath
	cfg.EffectiveBaselinePath = effectivePath

	if cfg.WriteBaseline && cfg.EffectiveBaselinePath == "" {
		return Config{}, errors.New("--write-baseline requires an effective baseline path")
	}

	resolvedGit, err := resolveEffectiveGitSettings(cfg, ruleSet.ToRules().Git)
	if err != nil {
		return Config{}, err
	}

	cfg.GitMode = resolvedGit.Mode
	cfg.GitDiffTarget = resolvedGit.Diff
	cfg.GitAddedLinesOnly = resolvedGit.AddedLinesOnly
	cfg.GitignoreEnabled = resolvedGit.GitignoreEnabled

	return cfg, nil
}

func resolveEffectiveGitSettings(cfg Config, fromRuleSet rules.GitSettings) (rules.GitSettings, error) {
	resolved := defaultEffectiveGitSettings(fromRuleSet)
	applyGitCLIOverrides(cfg, &resolved)

	return resolved, validateEffectiveGitSettings(resolved)
}

func defaultEffectiveGitSettings(fromRuleSet rules.GitSettings) rules.GitSettings {
	resolved := rules.GitSettings{
		Mode:             fromRuleSet.Mode,
		Diff:             strings.TrimSpace(fromRuleSet.Diff),
		AddedLinesOnly:   fromRuleSet.AddedLinesOnly,
		GitignoreEnabled: fromRuleSet.GitignoreEnabled,
	}
	if resolved.Mode == "" {
		resolved.Mode = "off"
	}

	return resolved
}

func applyGitCLIOverrides(cfg Config, resolved *rules.GitSettings) {
	if cfg.GitModeSet {
		resolved.Mode = cfg.GitMode
	}
	if cfg.GitDiffSet {
		resolved.Mode = "diff"
		resolved.Diff = cfg.GitDiffTarget
	}
	if cfg.GitAddedLinesOnlySet {
		resolved.AddedLinesOnly = cfg.GitAddedLinesOnly
	}
	if cfg.NoGitignore {
		resolved.GitignoreEnabled = false
	}
	if resolved.Mode != "diff" {
		resolved.Diff = ""
	}
}

func validateEffectiveGitSettings(resolved rules.GitSettings) error {
	if !isValidGitMode(resolved.Mode) {
		return errors.New("--git-mode must be one of off, staged, diff")
	}
	if resolved.Mode == "diff" && resolved.Diff == "" {
		return errors.New("effective --git-mode=diff requires --git-diff")
	}
	if resolved.AddedLinesOnly && resolved.Mode == "off" {
		return errors.New("--git-added-lines-only is valid only when --git-mode=staged|diff")
	}

	return nil
}

func buildGitRequest(settings rules.GitSettings) *scan.GitSelectionRequest {
	mode := strings.TrimSpace(settings.Mode)
	if mode == "" || mode == "off" {
		return nil
	}

	return &scan.GitSelectionRequest{
		Mode:             mode,
		DiffTarget:       strings.TrimSpace(settings.Diff),
		AddedLinesOnly:   settings.AddedLinesOnly,
		GitignoreEnabled: settings.GitignoreEnabled,
	}
}

var checkGitCapabilities = git.CheckCapabilities

var selectGitCandidateFiles = git.SelectCandidateFiles

var selectGitAddedLines = git.SelectAddedLines

func buildScanHooks(cfg Config, request scan.Request) (hooks.Registry, hooks.RunContext) {
	if request.Git == nil {
		return hooks.NewRegistry(), hooks.RunContext{}
	}

	context := hooks.RunContext{
		Mode:             request.Git.Mode,
		DiffTarget:       request.Git.DiffTarget,
		WorkingDir:       resolveGitWorkingDir(cfg.Roots),
		AddedLinesOnly:   request.Git.AddedLinesOnly,
		GitignoreEnabled: request.Git.GitignoreEnabled,
	}
	provider := git.NewHookProvider(checkGitCapabilities, selectGitCandidateFiles, selectGitAddedLines)

	return hooks.NewRegistry(provider), context
}

func applyCandidateScope(request *scan.Request, context *hooks.RunContext, scope hooks.CandidateScope) {
	if request.Git == nil {
		return
	}

	request.Git.CandidateFiles = append([]string{}, scope.CandidateFiles...)

	request.Git.AddedLinesByFile = cloneAddedLinesByFile(scope.AddedLinesByFile)
	context.AddedLinesByFile = request.Git.AddedLinesByFile
}

func applyIgnoreAugmentation(request *scan.Request, augmentation hooks.IgnoreAugmentation) {
	if len(augmentation.Files) == 0 {
		return
	}

	request.Ignore.Files = mergeIgnoreFiles(augmentation.Files, request.Ignore.Files)
}

func mergeIgnoreFiles(primary []string, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]string, 0, len(primary)+len(secondary))

	appendUnique := func(values []string) {
		for _, value := range values {
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}

			seen[value] = struct{}{}
			merged = append(merged, value)
		}
	}

	appendUnique(primary)
	appendUnique(secondary)

	return merged
}

func cloneAddedLinesByFile(source map[string]map[int]struct{}) map[string]map[int]struct{} {
	if len(source) == 0 {
		return nil
	}

	cloned := make(map[string]map[int]struct{}, len(source))
	for filePath, lineSet := range source {
		if len(lineSet) == 0 {
			continue
		}

		copiedLineSet := make(map[int]struct{}, len(lineSet))
		for line := range lineSet {
			copiedLineSet[line] = struct{}{}
		}
		cloned[filePath] = copiedLineSet
	}

	if len(cloned) == 0 {
		return nil
	}

	return cloned
}

func applyAfterMatchHooks(result scan.Result, registry hooks.Registry, context hooks.RunContext) (scan.Result, error) {
	if len(result.Matches) == 0 {
		return result, nil
	}

	filtered := make([]scan.Match, 0, len(result.Matches))
	for _, match := range result.Matches {
		keep, err := registry.AfterMatch(context, hooks.MatchContext{FilePath: match.FilePath, Line: match.Line})
		if err != nil {
			return scan.Result{}, err
		}
		if keep {
			filtered = append(filtered, match)
		}
	}

	result.Matches = filtered
	result.Stats.Matches = len(result.Matches)

	return result, nil
}

func resolveGitWorkingDir(roots []string) string {
	if len(roots) == 0 {
		return "."
	}

	firstRoot := filepath.Clean(roots[0])
	info, err := os.Stat(firstRoot)
	if err != nil {
		return firstRoot
	}
	if info.IsDir() {
		return firstRoot
	}

	return filepath.Dir(firstRoot)
}

func applyBaselineMode(cfg Config, result scan.Result, failOn string) (scan.Result, string, error) {
	if cfg.WriteBaseline {
		generation := baseline.Generate(result.Matches)
		if err := baseline.Write(cfg.EffectiveBaselinePath, generation.Document); err != nil {
			return scan.Result{}, "", err
		}

		return result, "", nil
	}

	if cfg.EffectiveBaselinePath == "" {
		return result, failOn, nil
	}

	document, err := baseline.Load(cfg.EffectiveBaselinePath)
	if err != nil {
		return scan.Result{}, "", err
	}

	comparison := baseline.Compare(result.Matches, document)
	result = applyBaselineComparison(result, comparison)

	return result, failOn, nil
}

func applyBaselineComparison(result scan.Result, comparison baseline.Comparison) scan.Result {
	result.Matches = append([]scan.Match{}, comparison.Regressions...)
	result.Stats.Matches = len(result.Matches)

	return result
}

func renderOutputs(
	formats []string,
	ruleset []rules.Rule,
	cfg Config,
	consoleColors output.ConsoleColorSettings,
	result scan.Result,
	out *bytes.Buffer,
) error {
	registry, err := outputRegistry(ruleset, consoleColors)
	if err != nil {
		return err
	}

	for _, format := range formats {
		formatter, err := registry.ResolveName(format)
		if err != nil {
			return err
		}
		if err := renderFormat(formatter, cfg, ruleset, result, out); err != nil {
			return err
		}
	}

	return nil
}

var outputRegistry = defaultOutputRegistry

func defaultOutputRegistry(ruleset []rules.Rule, consoleColors output.ConsoleColorSettings) (*output.Registry, error) {
	return output.NewRegistry(
		output.ConsoleFormatter{ColorSettings: consoleColors},
		output.JSONFormatter{},
		output.SARIFFormatter{Rules: ruleset},
	)
}

func renderFormat(
	formatter output.Formatter,
	cfg Config,
	ruleset []rules.Rule,
	result scan.Result,
	out *bytes.Buffer,
) error {
	switch formatter.Name() {
	case "console":
		return formatter.Write(result, out)
	case "json":
		return writeJSONOutput(cfg, result, out)
	case "sarif":
		return writeSARIFOutput(cfg, result, ruleset, out)
	default:
		return fmt.Errorf("invalid format: %s", formatter.Name())
	}
}

func writeJSONOutput(cfg Config, result scan.Result, out *bytes.Buffer) error {
	if cfg.OutJSON == "" {
		if len(cfg.Formats) != 1 {
			return errors.New("--out-json is required when requesting json with multiple formats")
		}

		return output.WriteJSON(result, out)
	}

	return writeJSONFile(cfg.OutJSON, result)
}

func writeSARIFOutput(cfg Config, result scan.Result, ruleset []rules.Rule, out *bytes.Buffer) error {
	if cfg.OutSARIF == "" {
		if len(cfg.Formats) != 1 {
			return errors.New("--out-sarif is required when requesting sarif with multiple formats")
		}

		return output.WriteSARIF(result, ruleset, out)
	}

	return writeSARIFFile(cfg.OutSARIF, result, ruleset)
}

func writeJSONFile(path string, result scan.Result) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultFileMode) //#nosec G304
	if err != nil {
		return err
	}
	if err := output.WriteJSON(result, file); err != nil {
		_ = file.Close()

		return err
	}

	return file.Close()
}

func writeSARIFFile(path string, result scan.Result, ruleSet []rules.Rule) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultFileMode) //#nosec G304
	if err != nil {
		return err
	}
	if err := output.WriteSARIF(result, ruleSet, file); err != nil {
		_ = file.Close()

		return err
	}

	return file.Close()
}

func hasFailingMatch(matches []scan.Match, failOn string) bool {
	threshold := severityRank(failOn)
	for _, match := range matches {
		if severityRank(match.Severity) <= threshold {
			return true
		}
	}

	return false
}

func exitCodeForFailOn(matches []scan.Match, failOn string) int {
	if hasFailingMatch(matches, failOn) {
		return exitCodeFailOn
	}

	return 0
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

func applyRuleSetOverrides(cfg Config, effective *rules.RuleSet) {
	if len(cfg.Include) > 0 {
		effective.Include = append([]string{}, cfg.Include...)
	}
	if len(cfg.Exclude) > 0 {
		effective.Exclude = append([]string{}, cfg.Exclude...)
	}
	if cfg.FailOnSeverity != "" {
		effective.FailOn = &cfg.FailOnSeverity
	}
	if cfg.NoIgnoreFiles {
		ignoreFilesDisabled := false
		effective.IgnoreFilesEnabled = &ignoreFilesDisabled
	}
}

func buildEffectiveRules(cfg Config, effective rules.RuleSet) []rules.Rule {
	if len(cfg.Include) == 0 && len(cfg.Exclude) == 0 {
		return effective.Rules
	}

	effectiveRules := make([]rules.Rule, len(effective.Rules))
	for i, rule := range effective.Rules {
		copied := rule
		if len(cfg.Include) > 0 {
			copied.Paths = append([]string{}, effective.Include...)
		}
		if len(cfg.Exclude) > 0 {
			copied.Exclude = append([]string{}, effective.Exclude...)
		}
		effectiveRules[i] = copied
	}

	return effectiveRules
}

func resolveConcurrency(cfg Config, rulesetConcurrency *int) int {
	if !cfg.ConcurrencySet && rulesetConcurrency != nil {
		return *rulesetConcurrency
	}

	return cfg.Concurrency
}

func resolveFailOn(failOn *string) string {
	if failOn == nil {
		return ""
	}

	return *failOn
}

func resolveConsoleColorSettings(effective rules.RuleSet) output.ConsoleColorSettings {
	settings := output.ConsoleColorSettings{
		Enabled: true,
		Source:  output.ConsoleColorSourceDefault,
	}
	if effective.ConsoleColorsEnabled != nil {
		settings.Enabled = *effective.ConsoleColorsEnabled
		settings.Source = output.ConsoleColorSourceConfig
	}

	if envValue, ok := os.LookupEnv("NO_COLOR"); ok && envValue != "" {
		settings.Enabled = false
		settings.Source = output.ConsoleColorSourceEnv
	}

	return settings
}

func resolveIgnoreSettings(effective rules.RuleSet, gitSettings rules.GitSettings) scan.IgnoreSettings {
	settings := scan.IgnoreSettings{
		Enabled: true,
		Files:   []string{".ignore", ".reglintignore"},
	}
	if effective.IgnoreFilesEnabled != nil {
		settings.Enabled = *effective.IgnoreFilesEnabled
	}
	if len(effective.IgnoreFiles) > 0 {
		settings.Files = append([]string{}, effective.IgnoreFiles...)
	}
	if !gitSettings.GitignoreEnabled {
		settings.Files = removeIgnoreFile(settings.Files, ".gitignore")
	}
	if settings.Enabled && gitSettings.GitignoreEnabled {
		settings.Files = mergeIgnoreFiles([]string{".gitignore"}, settings.Files)
	}

	return settings
}

func removeIgnoreFile(files []string, target string) []string {
	if len(files) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if file == target {
			continue
		}
		filtered = append(filtered, file)
	}

	return filtered
}

func wasFlagProvided(flagSet *flag.FlagSet, name string) bool {
	found := false
	flagSet.Visit(func(flagItem *flag.Flag) {
		if flagItem.Name == name {
			found = true
		}
	})

	return found
}

func resolveBaselinePaths(cfg Config, ruleSet config.RuleSet) (string, string, string, error) {
	ruleSetPath, err := resolveRuleSetBaselinePath(cfg.ConfigPath, ruleSet.Baseline)
	if err != nil {
		return "", "", "", err
	}

	cliPath, err := resolveCLIBaselinePath(cfg.BaselinePath)
	if err != nil {
		return "", "", "", err
	}

	effective := ruleSetPath
	if cliPath != "" {
		effective = cliPath
	}

	return cliPath, ruleSetPath, effective, nil
}

func resolveRuleSetBaselinePath(configPath string, baseline *string) (string, error) {
	if baseline == nil {
		return "", nil
	}

	trimmed := strings.TrimSpace(*baseline)
	if trimmed == "" {
		return "", nil
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}

	configDir := filepath.Dir(configPath)

	return filepath.Clean(filepath.Join(configDir, trimmed)), nil
}

func resolveCLIBaselinePath(pathValue string) (string, error) {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", nil
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve baseline path: %w", err)
	}

	return filepath.Clean(filepath.Join(cwd, trimmed)), nil
}
