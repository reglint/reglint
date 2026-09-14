package git_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reglint/reglint/internal/git"
	"github.com/reglint/reglint/internal/hooks"
)

func TestGitHookProviderModeOffIsNoOp(t *testing.T) {
	capabilityCalls := 0
	candidateCalls := 0
	addedLineCalls := 0

	provider := git.NewHookProvider(
		func(git.CapabilityRequest) error {
			capabilityCalls++

			return nil
		},
		func(git.CandidateSelectionRequest) ([]string, error) {
			candidateCalls++

			return []string{"pkg/a.go"}, nil
		},
		func(git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			addedLineCalls++

			return map[string]map[int]struct{}{"pkg/a.go": {1: {}}}, nil
		},
	)

	ctx := hooks.RunContext{Mode: "off", WorkingDir: "/repo"}
	if err := provider.OnCapabilitiesCheck(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, err := provider.BeforeCollectCandidates(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, err := provider.BeforeIgnoreEvaluation(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	keep, err := provider.AfterMatch(ctx, hooks.MatchContext{FilePath: "pkg/a.go", Line: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !keep {
		t.Fatal("expected match to be kept")
	}
	if capabilityCalls != 0 {
		t.Fatalf("expected no capability checks, got %d", capabilityCalls)
	}
	if candidateCalls != 0 {
		t.Fatalf("expected no candidate selections, got %d", candidateCalls)
	}
	if addedLineCalls != 0 {
		t.Fatalf("expected no added-line selections, got %d", addedLineCalls)
	}
}

func TestGitHookProviderCollectsCandidatesAndAddedLines(t *testing.T) {
	provider := git.NewHookProvider(
		func(git.CapabilityRequest) error {
			return nil
		},
		func(request git.CandidateSelectionRequest) ([]string, error) {
			if request.Mode != "diff" {
				t.Fatalf("expected mode diff, got %q", request.Mode)
			}
			if request.DiffTarget != "HEAD~1..HEAD" {
				t.Fatalf("expected diff target HEAD~1..HEAD, got %q", request.DiffTarget)
			}

			return []string{"pkg/b.go", "pkg/a.go"}, nil
		},
		func(request git.CandidateSelectionRequest) (map[string]map[int]struct{}, error) {
			if request.Mode != "diff" {
				t.Fatalf("expected mode diff, got %q", request.Mode)
			}

			return map[string]map[int]struct{}{
				"pkg/a.go": {1: {}, 2: {}},
			}, nil
		},
	)

	scope, err := provider.BeforeCollectCandidates(hooks.RunContext{
		Mode:           "diff",
		DiffTarget:     "HEAD~1..HEAD",
		WorkingDir:     "/repo",
		AddedLinesOnly: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(scope.CandidateFiles, []string{"pkg/b.go", "pkg/a.go"}) {
		t.Fatalf("unexpected candidate files: %v", scope.CandidateFiles)
	}
	if _, ok := scope.AddedLinesByFile["pkg/a.go"][1]; !ok {
		t.Fatal("expected added line 1 for pkg/a.go")
	}
	if _, ok := scope.AddedLinesByFile["pkg/a.go"][2]; !ok {
		t.Fatal("expected added line 2 for pkg/a.go")
	}
}

func TestGitHookProviderBeforeIgnoreEvaluation(t *testing.T) {
	provider := git.NewHookProvider(nil, nil, nil)

	augmentation, err := provider.BeforeIgnoreEvaluation(hooks.RunContext{Mode: "staged", GitignoreEnabled: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(augmentation.Files, []string{".gitignore"}) {
		t.Fatalf("unexpected ignore augmentation: %v", augmentation.Files)
	}

	augmentation, err = provider.BeforeIgnoreEvaluation(hooks.RunContext{Mode: "staged", GitignoreEnabled: false})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(augmentation.Files) != 0 {
		t.Fatalf("expected no ignore augmentation, got %v", augmentation.Files)
	}
}

func TestGitHookProviderAfterMatchFiltersAddedLines(t *testing.T) {
	provider := git.NewHookProvider(nil, nil, nil)

	ctx := hooks.RunContext{
		Mode:           "staged",
		AddedLinesOnly: true,
		AddedLinesByFile: map[string]map[int]struct{}{
			"pkg/a.go": {3: {}, 7: {}},
		},
	}

	keep, err := provider.AfterMatch(ctx, hooks.MatchContext{FilePath: "pkg/a.go", Line: 7})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !keep {
		t.Fatal("expected line 7 to be kept")
	}

	keep, err = provider.AfterMatch(ctx, hooks.MatchContext{FilePath: "pkg/a.go", Line: 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if keep {
		t.Fatal("expected line 2 to be dropped")
	}

	keep, err = provider.AfterMatch(ctx, hooks.MatchContext{FilePath: "pkg/other.go", Line: 7})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if keep {
		t.Fatal("expected unmatched file to be dropped")
	}
}

func TestGitHookProviderPropagatesDependencyErrors(t *testing.T) {
	provider := git.NewHookProvider(
		func(git.CapabilityRequest) error { return errors.New("capability failure") },
		func(git.CandidateSelectionRequest) ([]string, error) {
			return nil, errors.New("candidate failure")
		},
		nil,
	)

	err := provider.OnCapabilitiesCheck(hooks.RunContext{Mode: "staged"})
	if err == nil {
		t.Fatal("expected capability error, got nil")
	}
	if err.Error() != "capability failure" {
		t.Fatalf("unexpected capability error: %v", err)
	}

	_, err = provider.BeforeCollectCandidates(hooks.RunContext{Mode: "staged"})
	if err == nil {
		t.Fatal("expected candidate selection error, got nil")
	}
	if err.Error() != "candidate failure" {
		t.Fatalf("unexpected candidate selection error: %v", err)
	}
}
