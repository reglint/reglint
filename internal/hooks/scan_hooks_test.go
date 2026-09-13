package hooks_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reglint/reglint/internal/hooks"
)

type stubProvider struct {
	onCapabilities      func(hooks.RunContext) error
	beforeCollect       func(hooks.RunContext) (hooks.CandidateScope, error)
	beforeIgnore        func(hooks.RunContext) (hooks.IgnoreAugmentation, error)
	afterMatch          func(hooks.RunContext, hooks.MatchContext) (bool, error)
	onCapabilitiesCalls int
	beforeCollectCalls  int
	beforeIgnoreCalls   int
	afterMatchCalls     int
}

func (s *stubProvider) OnCapabilitiesCheck(ctx hooks.RunContext) error {
	s.onCapabilitiesCalls++
	if s.onCapabilities == nil {
		return nil
	}

	return s.onCapabilities(ctx)
}

func (s *stubProvider) BeforeCollectCandidates(ctx hooks.RunContext) (hooks.CandidateScope, error) {
	s.beforeCollectCalls++
	if s.beforeCollect == nil {
		return hooks.CandidateScope{}, nil
	}

	return s.beforeCollect(ctx)
}

func (s *stubProvider) BeforeIgnoreEvaluation(ctx hooks.RunContext) (hooks.IgnoreAugmentation, error) {
	s.beforeIgnoreCalls++
	if s.beforeIgnore == nil {
		return hooks.IgnoreAugmentation{}, nil
	}

	return s.beforeIgnore(ctx)
}

func (s *stubProvider) AfterMatch(ctx hooks.RunContext, match hooks.MatchContext) (bool, error) {
	s.afterMatchCalls++
	if s.afterMatch == nil {
		return true, nil
	}

	return s.afterMatch(ctx, match)
}

func TestRegistryRunsCapabilitiesChecksInProviderOrder(t *testing.T) {
	order := make([]string, 0, 2)
	providerA := &stubProvider{onCapabilities: func(hooks.RunContext) error {
		order = append(order, "a")

		return nil
	}}
	providerB := &stubProvider{onCapabilities: func(hooks.RunContext) error {
		order = append(order, "b")

		return nil
	}}

	registry := hooks.NewRegistry(providerA, providerB)
	err := registry.OnCapabilitiesCheck(hooks.RunContext{Mode: "staged"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(order, []string{"a", "b"}) {
		t.Fatalf("expected provider order [a b], got %v", order)
	}
}

func TestRegistryBeforeCollectMergesDeterministically(t *testing.T) {
	providerA := &stubProvider{beforeCollect: func(hooks.RunContext) (hooks.CandidateScope, error) {
		return hooks.CandidateScope{
			CandidateFiles: []string{"pkg/b.go", "pkg/a.go"},
			AddedLinesByFile: map[string]map[int]struct{}{
				"pkg/a.go": {2: {}},
				"pkg/b.go": {8: {}},
			},
		}, nil
	}}
	providerB := &stubProvider{beforeCollect: func(hooks.RunContext) (hooks.CandidateScope, error) {
		return hooks.CandidateScope{
			CandidateFiles: []string{"pkg/a.go", "pkg/c.go"},
			AddedLinesByFile: map[string]map[int]struct{}{
				"pkg/a.go": {1: {}},
				"pkg/c.go": {4: {}},
			},
		}, nil
	}}

	registry := hooks.NewRegistry(providerA, providerB)
	scope, err := registry.BeforeCollectCandidates(hooks.RunContext{Mode: "diff", DiffTarget: "HEAD~1..HEAD"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wantFiles := []string{"pkg/a.go", "pkg/b.go", "pkg/c.go"}
	if !reflect.DeepEqual(scope.CandidateFiles, wantFiles) {
		t.Fatalf("expected candidate files %v, got %v", wantFiles, scope.CandidateFiles)
	}

	assertAddedLines(t, scope.AddedLinesByFile, "pkg/a.go", []int{1, 2})
	assertAddedLines(t, scope.AddedLinesByFile, "pkg/b.go", []int{8})
	assertAddedLines(t, scope.AddedLinesByFile, "pkg/c.go", []int{4})
}

func TestRegistryBeforeIgnoreMergesInProviderOrder(t *testing.T) {
	providerA := &stubProvider{beforeIgnore: func(hooks.RunContext) (hooks.IgnoreAugmentation, error) {
		return hooks.IgnoreAugmentation{Files: []string{".gitignore", ".ignore"}}, nil
	}}
	providerB := &stubProvider{beforeIgnore: func(hooks.RunContext) (hooks.IgnoreAugmentation, error) {
		return hooks.IgnoreAugmentation{Files: []string{".gitignore", ".reglintignore"}}, nil
	}}

	registry := hooks.NewRegistry(providerA, providerB)
	augmentation, err := registry.BeforeIgnoreEvaluation(hooks.RunContext{Mode: "staged"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{".gitignore", ".ignore", ".reglintignore"}
	if !reflect.DeepEqual(augmentation.Files, want) {
		t.Fatalf("expected ignore files %v, got %v", want, augmentation.Files)
	}
}

func TestRegistryStopsOnFirstHookError(t *testing.T) {
	providerA := &stubProvider{beforeCollect: func(hooks.RunContext) (hooks.CandidateScope, error) {
		return hooks.CandidateScope{}, errors.New("boom")
	}}
	providerB := &stubProvider{beforeCollect: func(hooks.RunContext) (hooks.CandidateScope, error) {
		return hooks.CandidateScope{CandidateFiles: []string{"pkg/a.go"}}, nil
	}}

	registry := hooks.NewRegistry(providerA, providerB)
	_, err := registry.BeforeCollectCandidates(hooks.RunContext{Mode: "staged"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if providerB.beforeCollectCalls != 0 {
		t.Fatalf("expected second provider not to run, got %d calls", providerB.beforeCollectCalls)
	}
}

func TestRegistryAfterMatchDropsWhenAnyProviderDrops(t *testing.T) {
	providerA := &stubProvider{afterMatch: func(hooks.RunContext, hooks.MatchContext) (bool, error) {
		return true, nil
	}}
	providerB := &stubProvider{afterMatch: func(hooks.RunContext, hooks.MatchContext) (bool, error) {
		return false, nil
	}}

	registry := hooks.NewRegistry(providerA, providerB)
	keep, err := registry.AfterMatch(
		hooks.RunContext{Mode: "diff"},
		hooks.MatchContext{FilePath: "pkg/a.go", Line: 10},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if keep {
		t.Fatal("expected match to be dropped")
	}
}

func TestRegistryEnabled(t *testing.T) {
	if hooks.NewRegistry().Enabled() {
		t.Fatal("expected empty registry to be disabled")
	}
	if !hooks.NewRegistry(&stubProvider{}).Enabled() {
		t.Fatal("expected non-empty registry to be enabled")
	}
}

func assertAddedLines(t *testing.T, addedLinesByFile map[string]map[int]struct{}, filePath string, want []int) {
	t.Helper()

	lineSet, ok := addedLinesByFile[filePath]
	if !ok {
		t.Fatalf("expected added lines for %s", filePath)
	}
	if len(lineSet) != len(want) {
		t.Fatalf("expected %d added lines for %s, got %d", len(want), filePath, len(lineSet))
	}
	for _, line := range want {
		if _, exists := lineSet[line]; !exists {
			t.Fatalf("expected line %d for %s", line, filePath)
		}
	}
}
