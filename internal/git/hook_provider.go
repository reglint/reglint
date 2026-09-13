package git

import "github.com/reglint/reglint/internal/hooks"

// HookProvider wires Git adapter behaviors into scan hook contracts.
type HookProvider struct {
	checkCapabilities func(CapabilityRequest) error
	selectCandidate   func(CandidateSelectionRequest) ([]string, error)
	selectAddedLines  func(CandidateSelectionRequest) (map[string]map[int]struct{}, error)
}

// NewHookProvider creates a Git hook provider with optional dependency overrides.
func NewHookProvider(
	checkCapabilities func(CapabilityRequest) error,
	selectCandidate func(CandidateSelectionRequest) ([]string, error),
	selectAddedLines func(CandidateSelectionRequest) (map[string]map[int]struct{}, error),
) *HookProvider {
	provider := &HookProvider{
		checkCapabilities: checkCapabilities,
		selectCandidate:   selectCandidate,
		selectAddedLines:  selectAddedLines,
	}
	if provider.checkCapabilities == nil {
		provider.checkCapabilities = CheckCapabilities
	}
	if provider.selectCandidate == nil {
		provider.selectCandidate = SelectCandidateFiles
	}
	if provider.selectAddedLines == nil {
		provider.selectAddedLines = SelectAddedLines
	}

	return provider
}

// OnCapabilitiesCheck validates Git runtime requirements for enabled modes.
func (provider *HookProvider) OnCapabilitiesCheck(ctx hooks.RunContext) error {
	if ctx.Mode == "" || ctx.Mode == "off" {
		return nil
	}

	return provider.checkCapabilities(CapabilityRequest{Mode: ctx.Mode, WorkingDir: ctx.WorkingDir})
}

// BeforeCollectCandidates resolves Git-backed candidate files and added-line sets.
func (provider *HookProvider) BeforeCollectCandidates(ctx hooks.RunContext) (hooks.CandidateScope, error) {
	if ctx.Mode == "" || ctx.Mode == "off" {
		return hooks.CandidateScope{}, nil
	}

	request := CandidateSelectionRequest{
		Mode:       ctx.Mode,
		DiffTarget: ctx.DiffTarget,
		WorkingDir: ctx.WorkingDir,
	}

	candidateFiles, err := provider.selectCandidate(request)
	if err != nil {
		return hooks.CandidateScope{}, err
	}

	scope := hooks.CandidateScope{CandidateFiles: append([]string{}, candidateFiles...)}
	if !ctx.AddedLinesOnly {
		return scope, nil
	}

	addedLinesByFile, err := provider.selectAddedLines(request)
	if err != nil {
		return hooks.CandidateScope{}, err
	}
	scope.AddedLinesByFile = cloneAddedLinesByFile(addedLinesByFile)

	return scope, nil
}

// BeforeIgnoreEvaluation contributes optional .gitignore augmentation.
func (provider *HookProvider) BeforeIgnoreEvaluation(ctx hooks.RunContext) (hooks.IgnoreAugmentation, error) {
	if ctx.Mode == "" || ctx.Mode == "off" || !ctx.GitignoreEnabled {
		return hooks.IgnoreAugmentation{}, nil
	}

	return hooks.IgnoreAugmentation{Files: []string{".gitignore"}}, nil
}

// AfterMatch applies added-lines-only filtering when enabled.
func (provider *HookProvider) AfterMatch(ctx hooks.RunContext, match hooks.MatchContext) (bool, error) {
	if !ctx.AddedLinesOnly || ctx.Mode == "" || ctx.Mode == "off" {
		return true, nil
	}

	lineSet, ok := ctx.AddedLinesByFile[match.FilePath]
	if !ok {
		return false, nil
	}

	_, keep := lineSet[match.Line]

	return keep, nil
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
