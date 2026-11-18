package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EFullWorkflowExistsForNightlyAndManualRuns(t *testing.T) {
	t.Parallel()

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	workflowPath := filepath.Join(moduleRoot, ".github", "workflows", "e2e-full.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read e2e full workflow: %v", err)
	}

	workflow := string(content)
	requiredSnippets := []string{
		"schedule:",
		"workflow_dispatch:",
		"cron:",
		"e2e-full:",
		"run: devenv shell -- make test-e2e",
	}

	for _, snippet := range requiredSnippets {
		if strings.Contains(workflow, snippet) {
			continue
		}

		t.Fatalf("expected %s to contain %q", workflowPath, snippet)
	}

	forbiddenSnippets := []string{"pull_request:", "push:"}
	for _, snippet := range forbiddenSnippets {
		if !strings.Contains(workflow, snippet) {
			continue
		}

		t.Fatalf("expected %s to omit %q", workflowPath, snippet)
	}
}
