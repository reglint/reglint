package ignore_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/reglint/reglint/internal/ignore"
)

func TestLoadOrdersRulesByDirectoryAndFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".ignore"), "root-rule\n")
	writeFile(t, filepath.Join(root, ".reglintignore"), "root-reglint\n")
	writeFile(t, filepath.Join(root, "a", ".ignore"), "a-rule\n")
	writeFile(t, filepath.Join(root, "a", ".reglintignore"), "a-reglint\n")
	writeFile(t, filepath.Join(root, "a", "sub", ".ignore"), "a-sub-rule\n")
	writeFile(t, filepath.Join(root, "b", ".ignore"), "b-rule\n")

	rules, err := ignore.Load(root, []string{".ignore", ".reglintignore"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []struct {
		source  string
		baseDir string
		pattern string
	}{
		{source: ".ignore", baseDir: ".", pattern: "root-rule"},
		{source: ".reglintignore", baseDir: ".", pattern: "root-reglint"},
		{source: "a/.ignore", baseDir: "a", pattern: "a-rule"},
		{source: "a/.reglintignore", baseDir: "a", pattern: "a-reglint"},
		{source: "a/sub/.ignore", baseDir: "a/sub", pattern: "a-sub-rule"},
		{source: "b/.ignore", baseDir: "b", pattern: "b-rule"},
	}

	if len(rules) != len(expected) {
		t.Fatalf("expected %d rules, got %d", len(expected), len(rules))
	}

	for i, want := range expected {
		rule := rules[i]
		if rule.Source != want.source {
			t.Fatalf("expected source %q, got %q", want.source, rule.Source)
		}
		if rule.BaseDir != want.baseDir {
			t.Fatalf("expected baseDir %q, got %q", want.baseDir, rule.BaseDir)
		}
		if rule.Pattern != want.pattern {
			t.Fatalf("expected pattern %q, got %q", want.pattern, rule.Pattern)
		}
	}
}

func TestLoadReturnsErrorOnUnreadableIgnoreFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions are unreliable on Windows")
	}

	root := t.TempDir()
	path := filepath.Join(root, ".ignore")
	writeFile(t, path, "rule\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer func() {
		_ = os.Chmod(path, 0644)
	}()

	_, err := ignore.Load(root, []string{".ignore"})
	if err == nil {
		t.Fatal("expected error for unreadable ignore file")
	}
}

func TestLoadSkipsMissingIgnoreFileNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".reglintignore"), "rule\n")

	rules, err := ignore.Load(root, []string{".ignore", ".reglintignore"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Source != ".reglintignore" {
		t.Fatalf("expected source %q, got %q", ".reglintignore", rules[0].Source)
	}
}

func TestLoadReturnsErrorOnInvalidPattern(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".ignore"), "[\n")

	_, err := ignore.Load(root, []string{".ignore"})
	if err == nil {
		t.Fatal("expected error for invalid ignore pattern")
	}
}

func TestLoadSkipsIgnoreFileDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".ignore"), 0755); err != nil {
		t.Fatalf("failed to create ignore directory: %v", err)
	}
	writeFile(t, filepath.Join(root, ".reglintignore"), "rule\n")

	rules, err := ignore.Load(root, []string{".ignore", ".reglintignore"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestLoadReturnsErrorOnUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions are unreliable on Windows")
	}

	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatalf("failed to chmod directory: %v", err)
	}
	defer func() {
		_ = os.Chmod(blocked, 0755)
	}()

	_, err := ignore.Load(root, []string{".ignore"})
	if err == nil {
		t.Fatal("expected error for unreadable directory")
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}
